package pkg

import "testing"

func TestNewCostAnalysis(t *testing.T) {
	tests := []struct {
		name string
		priceLocation string
		expected      *CostAnalysis
		expectedError bool
	}{
		{
			name: "blank",
			priceLocation: "",
			expected:      nil,
			expectedError: true,
		},
		{
			name: "nonexistent price path",
			priceLocation: "testdata/i_dont_exist.json",
			expected:      nil,
			expectedError: true,
		},
		{
			name: "malformed json local",
			priceLocation: "testdata/im_not_json.json",
			expected: nil,
			expectedError: true,
		},
		{
			name: "valid local pricing",
			priceLocation: "testdata/valid_pricing.json",
			expected: &CostAnalysis{
				priceSheetPath: "testdata/valid_pricing.json",
				pricing: Pricing{
					"us-west1-a": {
						"us-west1-b": 0.01,
					},
					"us-west1-b": {
						"us-west1-a": 0.01,
					},
				},
			},
		},
		{
			name: "nonexistent url",
			priceLocation: "https://goo.bar/elmo.json",
			expected: nil,
			expectedError: true,
		},
		{
			name: "invalid remote pricing",
			priceLocation: ""
		},
	}
	for _, v := range tests {

	}
}

func TestCostAnalysis_CalculateEgress(t *testing.T) {

}
