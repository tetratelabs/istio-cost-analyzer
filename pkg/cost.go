package pkg

type CostAnalysis struct {
	priceSheetPath string
}

func NewCostAnalysis(priceSheetPath string) (*CostAnalysis, error) {

}

/*
Json:

inter-region:
	within region (us) across zones
	us-west-1 -> us-west-2

intra-region:

*/
