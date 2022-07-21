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
	"github.com/olekukonko/tablewriter"
	"math"
	"os"
	"sort"
)

type Call struct {
	From         string
	FromWorkload string
	To           string
	ToWorkload   string
	CallCost     float64
	CallSize     uint64
}

func (c *Call) String() string {
	return fmt.Sprintf("%v (%v)->%v (%v) : %v", c.FromWorkload, c.From, c.ToWorkload, c.To, c.CallSize)
}

func (c *Call) StringCost() string {
	return fmt.Sprintf("%v (%v)->%v (%v) : $%v", c.FromWorkload, c.From, c.ToWorkload, c.To, c.CallCost)
}

func PrintCostTable(calls []*Call, total float64, details bool) {
	// print total
	fmt.Printf("\nTotal: %s\n\n", transformCost(total))
	if !details {
		printMinifiedCostTable(calls)
		return
	}
	// sort by cost
	sort.Slice(calls, func(i, j int) bool {
		return calls[i].CallCost > calls[j].CallCost
	})
	table := tablewriter.NewWriter(os.Stdout)
	headers := []string{"Source Service", "Source Locality", "Destination Service", "Destination Locality", "Transferred (MB)", "Cost"}
	table.SetHeader(headers)
	for _, v := range calls {
		values := []string{v.FromWorkload, v.From, v.ToWorkload, v.To, fmt.Sprintf("%f", float64(v.CallSize)/math.Pow(10, 6)), transformCost(v.CallCost)}
		table.Append(values)
	}
	kubernetesify(table)
	table.Render()
	fmt.Println()
}

func printMinifiedCostTable(calls []*Call) {
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
	headers := []string{"Source Service", "Source Locality", "Cost"}
	table.SetHeader(headers)
	for _, v := range callSlice {
		values := []string{v.FromWorkload, v.From, transformCost(v.CallCost)}
		table.Append(values)
	}
	kubernetesify(table)
	table.Render()
	fmt.Println()
}

func transformCost(cost float64) string {
	costStr := fmt.Sprintf("$%.2f", cost)
	if cost < 0.01 {
		costStr = "<$0.01"
	}
	if cost == 0 {
		costStr = "-"
	}
	return costStr
}

func kubernetesify(table *tablewriter.Table) {
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t") // pad with tabs
	table.SetNoWhiteSpace(true)
	table.SetBorder(false)
}

// PodCall represents raw pod data, not containing locality or cost information.
type PodCall struct {
	FromPod       string
	FromNamespace string
	FromWorkload  string
	ToPod         string
	ToWorkload    string
	ToNamespace   string
	CallSize      uint64
}
