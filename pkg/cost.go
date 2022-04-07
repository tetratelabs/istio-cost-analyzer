package pkg

import (
	"fmt"
	"k8s.io/apimachinery/pkg/util/json"
	"math"
	"os"
)

type CostAnalysis struct {
	priceSheetPath string
	pricing        Pricing
}

type Pricing map[string]map[string]float64

type PricingGCP struct {
	InterZone      map[string]interface{} `json:"inter-zone-intra-region"`
	InterRegion    map[string]interface{} `json:"inter-region-intra-continent"`
	InterContinent map[string]interface{} `json:"inter-continent"`
}

func NewCostAnalysis(priceSheetPath string) (*CostAnalysis, error) {
	pricing := Pricing{}
	data, err := os.ReadFile(priceSheetPath)
	if err != nil {
		fmt.Printf("unable to read file %v: %v", priceSheetPath, err)
		return nil, err
	}
	err = json.Unmarshal(data, &pricing)
	if err != nil {
		fmt.Printf("unable to unmarshal json into object: %v", err)
		return nil, err
	}
	return &CostAnalysis{
		priceSheetPath: priceSheetPath,
		pricing:        pricing,
	}, nil
}

func (c *CostAnalysis) CalculateEgress(calls []*Call) (float64, error) {
	totalCost := 0.00
	for i, v := range calls {
		cost := c.pricing[v.From][v.To] * (float64(v.CallSize) * math.Pow(10, -6))
		calls[i].CallCost = cost
		totalCost += cost
	}
	return totalCost, nil
}
