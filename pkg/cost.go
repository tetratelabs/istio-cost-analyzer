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

type PricingGCP struct {
	InterZone      map[string]interface{} `json:"inter-zone-intra-region"`
	InterRegion    map[string]interface{} `json:"inter-region-intra-continent"`
	InterContinent map[string]interface{} `json:"inter-continent"`
}

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
// todo should we return a multierror?
func (c *CostAnalysis) CalculateEgress(calls []*Call) (float64, error) {
	totalCost := 0.00
	for i, v := range calls {
		rate, ok := c.pricing[v.From][v.To]
		if !ok {
			fmt.Printf("unable to find rate for link between %v and %v, skipping...\n", v.From, v.To)
			continue
		}
		cost := rate * (float64(v.CallSize) * math.Pow(10, -6))
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
