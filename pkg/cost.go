// Copyright 2022 Tetrate
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pkg

import (
	"fmt"
	"io"
	"k8s.io/apimachinery/pkg/util/json"
	"math"
	"net/http"
	"net/url"
	"os"
)

type CostAnalysis struct {
	priceSheetPath string
	pricing        Pricing
}

type Pricing map[string]map[string]float64

func NewCostAnalysis(priceSheetLocation string) (*CostAnalysis, error) {
	pricing := Pricing{}
	var data []byte
	var err error
	if isValidUrl(priceSheetLocation) {
		resp, err := http.Get(priceSheetLocation)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		defer resp.Body.Close()
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
	} else {
		data, err = os.ReadFile(priceSheetLocation)
		if err != nil {
			fmt.Printf("unable to read file %v: %v", priceSheetLocation, err)
			return nil, err
		}
	}
	err = json.Unmarshal(data, &pricing)
	if err != nil {
		fmt.Printf("unable to unmarshal json into object: %v", err)
		return nil, err
	}
	return &CostAnalysis{
		priceSheetPath: priceSheetLocation,
		pricing:        pricing,
	}, nil
}

// CalculateEgress calculates the total egress costs based on the pricing structure
// in the CostAnalysis object. It stores the individual call prices in the calls object,
// along with returning a total cost as a float64. If an entry in calls doesn't correspond to
// the actual pricing structure, the function just skips that entry, instead of returning an error.
func (c *CostAnalysis) CalculateEgress(calls []*Call) (float64, error) {
	totalCost := 0.00
	fmt.Printf("calculating egress costs for %v call links\n", len(calls))
	for i, v := range calls {
		rate, ok := c.pricing[v.From][v.To]
		if !ok {
			fmt.Printf("unable to find rate for link between %v and %v, skipping...\n", v.From, v.To)
			continue
		}
		// 1 byte = 10^-9 gb
		cost := rate * (float64(v.CallSize) * math.Pow(10, -9))
		calls[i].CallCost = cost
		totalCost += cost
	}
	return totalCost, nil
}

func isValidUrl(toTest string) bool {
	_, err := url.ParseRequestURI(toTest)
	if err != nil {
		return false
	}

	u, err := url.Parse(toTest)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}

	return true
}
