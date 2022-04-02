package pkg

import (
	"fmt"
	"github.com/olekukonko/tablewriter"
	"math"
	"os"
	"sort"
)

type Call struct {
	From         *Locality
	FromWorkload string
	To           *Locality
	ToWorkload   string
	CallCost     float64
	CallSize     uint64
}

func (c *Call) String() string {
	return fmt.Sprintf("%v (%v)->%v (%v) : %v", c.FromWorkload, c.From.Zone, c.ToWorkload, c.To.Zone, c.CallSize)
}

func (c *Call) StringCost() string {
	return fmt.Sprintf("%v (%v)->%v (%v) : $%v", c.FromWorkload, c.From.Zone, c.ToWorkload, c.To.Zone, c.CallCost)
}

func PrintCostTable(calls []*Call, total float64, details bool) {
	// print total
	totalCost := "<$0.01"
	if total >= 0.01 {
		totalCost = fmt.Sprintf("%f", total)
	}
	fmt.Printf("\nTotal: %s\n\n", totalCost)
	if !details {
		printMinifiedCostTable(calls)
		return
	}
	// sort by cost
	sort.Slice(calls, func(i, j int) bool {
		return calls[i].CallCost > calls[j].CallCost
	})
	table := tablewriter.NewWriter(os.Stdout)
	headers := []string{"Source Workload", "Source Locality", "Destination Workload", "Destination Locality", "Transferred (MB)", "Cost"}
	table.SetHeader(headers)
	for _, v := range calls {
		values := []string{v.FromWorkload, v.From.Zone, v.ToWorkload, v.To.Zone, fmt.Sprintf("%f", float64(v.CallSize)/math.Pow(10, 6)), transformCost(v.CallCost)}
		table.Append(values)
	}
	table.SetBorder(false)
	table.Render()
	fmt.Println()
}

func printMinifiedCostTable(calls []*Call) {
	// collapse into set by source
	// todo should we group by source & locality or just source
	// 	foo in us-west1-b might be treated diff from foo in us-west1-a
	//  right now we group by just source, so all localities are grouped in the same row
	callBySource := make(map[string]Call)
	for i, v := range calls {
		if srcCall, ok := callBySource[v.FromWorkload]; !ok {
			callBySource[v.FromWorkload] = *calls[i]
		} else {
			srcCall.CallCost += callBySource[v.FromWorkload].CallCost
		}
	}
	callSlice := make([]Call, 0)
	for _, v := range callBySource {
		callSlice = append(callSlice, v)
	}
	// order by cost
	sort.Slice(callSlice, func(i, j int) bool {
		return callSlice[i].CallCost > callSlice[j].CallCost
	})
	// print
	table := tablewriter.NewWriter(os.Stdout)
	headers := []string{"Source Workload", "Source Locality", "Cost"}
	table.SetHeader(headers)
	for _, v := range callSlice {
		values := []string{v.FromWorkload, v.From.Zone, transformCost(v.CallCost)}
		table.Append(values)
	}
	table.SetBorder(false)
	table.Render()
	fmt.Println()
}

func transformCost(cost float64) string {
	costStr := fmt.Sprintf("%f", cost)
	if cost < 0.01 {
		costStr = "<$0.01"
	}
	if cost == 0 {
		costStr = "-"
	}
	return costStr
}

type PodCall struct {
	FromPod      string
	FromWorkload string
	ToPod        string
	ToWorkload   string
	CallSize     uint64
}

type Locality struct {
	Region  string
	Zone    string
	Subzone string
}
