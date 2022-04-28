package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
)

const awsUrl = "https://b0.p.awsstatic.com/pricing/2.0/meteredUnitMaps/datatransfer/USD/current/datatransfer.json?timestamp=1649448986885"

/*
Hacky as hell.

This doesn't work for all the regions (for some reason), but it works
for the major ones, which is good enough for now (?)

Don't use this in prod (please).

Not even going to bother documenting.
*/

var out string

func init() {
	flag.StringVar(&out, "out", "aws_pricing.json", "where to output pricing data")
}

func main() {
	resp, err := http.Get(awsUrl)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return
	}
	rates := map[string]interface{}{}
	if err := json.Unmarshal(body, &rates); err != nil {
		fmt.Println(err)
		return
	}
	regionsRefKeys := reflect.ValueOf(rates["regions"].(map[string]interface{})).MapKeys()
	regions := []string{}
	for _, v := range regionsRefKeys {
		regions = append(regions, v.Interface().(string))
	}
	ri := rates["sets"].(map[string]interface{})["DataTransfer InterRegion Outbound"].([]interface{})
	interRegionOutbound := []string{}
	for _, v := range ri {
		interRegionOutbound = append(interRegionOutbound, v.(string))
	}
	regionCodes := map[string]string{}
	regCodes, err := os.ReadFile("aws_regions.json")
	if err != nil {
		fmt.Println(err)
		return
	}
	err = json.Unmarshal(regCodes, &regionCodes)
	if err != nil {
		fmt.Println(err)
		return
	}
	awsRegNames := []string{}
	for c, _ := range regionCodes {
		awsRegNames = append(awsRegNames, c)
	}
	outRates := map[string]map[string]float64{}
	for _, reg := range regions {
		for _, outB := range interRegionOutbound {
			costInfo, ok := rates["regions"].(map[string]interface{})[reg].(map[string]interface{})[outB]
			if !ok {
				continue
			}
			rateStr, ok := costInfo.(map[string]interface{})["price"].(string)
			if !ok {
				fmt.Printf("No price field for region %v to %v", reg, outB)
				continue
			}
			rate, err := strconv.ParseFloat(rateStr, 64)
			if err != nil {
				fmt.Printf("couldnt convert rate: %v", err)
			}
			outB = strings.Replace(outB, "DataTransfer InterRegion Outbound to ", "", 1)
			matchedReg := strings.ReplaceAll(reg, "(", "")
			matchedReg = strings.ReplaceAll(matchedReg, ")", "")
			from := MostSimilar(matchedReg, awsRegNames)
			to := MostSimilar(outB, awsRegNames)
			if _, ok := outRates[regionCodes[from]]; !ok {
				outRates[regionCodes[from]] = map[string]float64{}
			}
			outRates[regionCodes[from]][regionCodes[to]] = rate
		}
	}
	output, err := json.MarshalIndent(outRates, "", "    ")
	if err != nil {
		fmt.Println(err)
		return
	}
	os.Remove(out)
	f, err := os.Create(out)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("outputting data to %v\n", out)
	f.Write(output)
}

func MostSimilar(want string, cont []string) string {
	maxScore := -1
	maxCont := ""
	for _, c := range cont {
		partsC := strings.Split(c, " ")
		partsW := strings.Split(want, " ")
		score := 0
		for _, w := range partsW {
			for _, pc := range partsC {
				if w == pc {
					score++
				}
			}
		}
		if score > maxScore {
			maxScore = score
			maxCont = c
		}
	}
	return maxCont
}
