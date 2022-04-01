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

// todo minified cost (group by source)

func PrintCostTable(calls []*Call, total float64) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Source Workload", "Source Locality", "Destination Workload", "Destination Locality", "Transferred (MB)", "Cost"})
	sort.Slice(calls, func(i, j int) bool {
		return calls[i].CallCost > calls[j].CallCost
	})
	for _, v := range calls {
		cost := fmt.Sprintf("%f", v.CallCost)
		if v.CallCost < 0.01 {
			cost = "<$0.01"
		}
		if v.CallCost == 0 {
			cost = "-"
		}
		table.Append([]string{v.FromWorkload, v.From.Zone, v.ToWorkload, v.To.Zone, fmt.Sprintf("%f", float64(v.CallSize)/math.Pow(10, 6)), cost})
	}
	table.SetBorder(false)
	table.SetFooter([]string{"", "", "", "", "total", fmt.Sprintf("$%f", total)})
	fmt.Println()
	table.Render()
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
