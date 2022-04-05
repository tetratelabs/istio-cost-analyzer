package pkg

import (
	"errors"
	"fmt"
	"k8s.io/apimachinery/pkg/util/json"
	"math"
	"os"
	"strconv"
	"strings"
)

type CostAnalysis struct {
	priceSheetPath string
	pricing        *Pricing
}

type Pricing struct {
	InterZone      map[string]interface{} `json:"inter-zone-intra-region"`
	InterRegion    map[string]interface{} `json:"inter-region-intra-continent"`
	InterContinent map[string]interface{} `json:"inter-continent"`
}

type InterPricing struct {
	Us           string `json:"us"`
	NorthAmerica string `json:"northamerica"`
	Eu           string `json:"eu"`
	Asia         string `json:"asia"`
	SouthAmerica string `json:"southamerica"`
	Australia    string `json:"australia"`
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
		pricing:        &pricing,
	}, nil
}

// CalculateEgress returns the total cost of all the calls passed in, and populates the
// array with the per-call costs.
func (c *CostAnalysis) CalculateEgress(calls []*Call) (float64, error) {
	totalCost := 0.00
	for i, v := range calls {
		// the way v.To/From is being used is kind of broken, todo fix
		dataRateStr := "0"
		var ok bool
		if c.GetContinent(v.To) == c.GetContinent(v.From) {
			if c.GetRegion(v.To) == c.GetRegion(v.From) {
				if c.GetZone(v.To) == c.GetZone(v.From) {
					// within same zone, no-op
				} else {
					// inter-zone rate for "to"
					dataRateInterface := c.pricing.InterZone[c.GetContinent(v.To)]
					dataRateStr, ok = dataRateInterface.(string)
					if !ok {
						return 0, errors.New(fmt.Sprintf("%v is not a string", dataRateStr))
					}
				}
			} else {
				// inter-region rate for "to"
				dataRateInterface := c.pricing.InterRegion[c.GetContinent(v.To)]
				dataRateStr, ok = dataRateInterface.(string)
				if !ok {
					return 0, errors.New(fmt.Sprintf("%v is not a string", dataRateStr))
				}
			}
		} else {
			// inter-continent rate for "to"
			dataRateInterface := c.pricing.InterContinent[c.GetContinent(v.To)]
			dataRateStr, ok = dataRateInterface.(string)
			if !ok {
				return 0, errors.New(fmt.Sprintf("%v is not a string", dataRateStr))
			}
		}
		rate, err := strconv.ParseFloat(dataRateStr, 64)
		if err != nil {
			fmt.Printf("unable to parse rate %v, skipping...", c.pricing.InterZone[c.GetZone(v.To)])
		}
		indivCost := rate * (float64(v.CallSize) / math.Pow(10, 9))
		totalCost += indivCost
		calls[i].CallCost = indivCost
	}
	return totalCost, nil
}

// <continent>-<region>-<zone>
func (c *CostAnalysis) GetContinent(region string) string {
	return strings.Split(region, "-")[0]
}

func (c *CostAnalysis) GetRegion(region string) string {
	return strings.Split(region, "-")[1]
}

func (c *CostAnalysis) GetZone(region string) string {
	return strings.Split(region, "-")[2]
}
