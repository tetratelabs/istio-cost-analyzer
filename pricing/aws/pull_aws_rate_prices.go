package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"reflect"
)

const awsUrl = "https://b0.p.awsstatic.com/pricing/2.0/meteredUnitMaps/datatransfer/USD/current/datatransfer.json?timestamp=1649448986885"

var (
	out string
)

func init() {
	flag.StringVar(&out, "out", "aws_rates.json", "file to output rates to")
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
	if err = json.Unmarshal(body, &rates); err != nil {
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
	fmt.Println(interRegionOutbound[0])

	for _, reg := range regions {
		for _, outB := range interRegionOutbound {
			costInfo, ok := rates["regions"].(map[string]interface{})[reg].(map[string]interface{})[outB]
			if !ok {
				fmt.Printf("skipping %v for region %v\n", outB, reg)
				continue
			}
			rate, ok := costInfo.(map[string]interface{})["price"]
			if !ok {
				fmt.Printf("skipping %v for region %v\n", outB, reg)
				continue
			}
			fmt.Println(rate)
		}
	}
	fmt.Println(rates["regions"].(map[string]interface{})["AWS GovCloud (US)"].(map[string]interface{})[interRegionOutbound[0]])
	//transferObj := DataTransferObject{}
	//json.Unmarshal(rates["Regions"].(map[string]interface{})[interRegionOutbound[0]], &transferObj)

	//fmt.Println(ri.([]interface{})[0].(string))
	//interRegionSet := ri.([]string)
	//for _, v := range interRegionSet {
	//	fmt.Println(v)
	//}
	//rates["regions"]["US East (Boston)"]["DataTransfer InterRegion Outbound to US West N California"]
	//k := reflect.ValueOf(rates["regions"]["US East (Boston)"]["DataTransfer InterRegion Outbound to US West N California"]).MapKeys()
	//for _, v := range k {
	//	fmt.Println(v)
	//}
	//fmt.Println(reflect.ValueOf(rates["regions"]).MapKeys())

	//var pretty bytes.Buffer
	//_ = json.Indent(&pretty, body, "", "\t")
	//_ = os.Remove(out)
	//f, err := os.Create(out)
	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}
	//defer f.Close()
	//_, err = f.Write(pretty.Bytes())
	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}
	//fmt.Printf("wrote rates to %v", out)
}
