package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

/*
PLEASE READ:

This is a hacky tool that converts gcp pricing with some structure to
a flat pricing scheme. PLEASE, PLEASE, PLEASE do not run this in production.
It will break.

Thanks
*/

var (
	outputFile string
	inputFile  string
)

type PricingGCP struct {
	InterZone      map[string]interface{} `json:"inter-zone-intra-region"`
	InterRegion    map[string]interface{} `json:"inter-region-intra-continent"`
	InterContinent map[string]interface{} `json:"inter-continent"`
}

func init() {
	flag.StringVar(&outputFile, "out", "default_pricing.json", "file to output transformed data")
	flag.StringVar(&inputFile, "in", "pricing.json", "input data")
	flag.Parse()
}

func main() {
	gcpRegions := map[string]string{
		`asia-east1`:              `abc`,
		`asia-east2`:              `abc`,
		`asia-northeast1`:         `abc`,
		`asia-northeast2`:         `abc`,
		`asia-northeast3`:         `abc`,
		`asia-south1`:             `abc`,
		`asia-southeast1`:         `abc`,
		`australia-southeast1`:    `abc`,
		`europe-north1`:           `abc`,
		`europe-west1`:            `bcd`,
		`europe-west2`:            `abc`,
		`europe-west3`:            `abc`,
		`europe-west4`:            `abc`,
		`europe-west6`:            `abc`,
		`northamerica-northeast1`: `abc`,
		`southamerica-east1`:      `abc`,
		`us-central1`:             `abcf`,
		`us-east1`:                `bcd`,
		`us-east4`:                `abc`,
		`us-west1`:                `abc`,
		`us-west2`:                `abc`,
		`us-west3`:                `abc`,
	}
	// if uc-central1, also zone f
	structuredPricing := PricingGCP{}
	data, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Printf("unable to read file %v: %v", inputFile, err)
		return
	}
	err = json.Unmarshal(data, &structuredPricing)
	if err != nil {
		fmt.Printf("unable to unmarshal json into object: %v", err)
		return
	}
	flatPricing := map[string]map[string]float64{}
	localities := make([]string, 0)
	for k, v := range gcpRegions {
		zones := []rune(v)
		for _, z := range zones {
			locality := k + "-" + string(z)
			localities = append(localities, locality)
		}
	}
	for _, v := range localities {
		flatPricing[v] = make(map[string]float64)
	}
	for _, from := range localities {
		for _, to := range localities {
			rateStr := ""
			if GetContinent(from) == GetContinent(to) {
				if GetRegion(from) == GetRegion(to) {
					if GetZone(from) == GetZone(to) {
						flatPricing[from][to] = 0
						continue
					} else {
						drI := structuredPricing.InterZone[GetContinent(to)]
						rateStr, _ = drI.(string)
					}
				} else {
					drI := structuredPricing.InterRegion[GetContinent(to)]
					rateStr, _ = drI.(string)
				}
			} else {
				drI := structuredPricing.InterContinent[GetContinent(to)]
				rateStr, _ = drI.(string)
			}
			rate, err := strconv.ParseFloat(rateStr, 64)
			if err != nil {
				fmt.Printf("cannot parse rate:%v\n", err)
			}
			flatPricing[from][to] = rate
		}
	}
	jsonStr, err := json.MarshalIndent(flatPricing, "", "    ")
	if err != nil {
		fmt.Println(err)
	}
	_ = os.Remove(outputFile)
	f, err := os.Create(outputFile)
	if err != nil {
		fmt.Println(err)
	}
	defer f.Close()
	_, err = f.Write(jsonStr)
	if err != nil {
		fmt.Println(err)
	}
}

func GetContinent(region string) string {
	return strings.Split(region, "-")[0]
}

func GetRegion(region string) string {
	return strings.Split(region, "-")[1]
}

func GetZone(region string) string {
	return strings.Split(region, "-")[2]
}
