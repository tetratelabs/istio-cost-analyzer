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
	"context"
	"fmt"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// CostAnalyzerProm holds the prometheus routines necessary to collect
// service<->service traffic data.
type CostAnalyzerProm struct {
	promEndpoint       string
	errChan            chan error
	client             api.Client
	localityMatch      string
	attemptsForwarding int
	// attemptsThreshold is the number of retry attempts we will make to port-forward
	attemptsThreshold int
}

// NewAnalyzerProm creates a prometheus client given the endpoint,
// and errors out if the endpoint is invalid.
func NewAnalyzerProm(promEndpoint, cloud string) (*CostAnalyzerProm, error) {
	client, err := api.NewClient(api.Config{
		Address: promEndpoint,
	})
	if err != nil {
		fmt.Printf("cannot initialize prom lib: %v", err)
		return nil, err
	}
	// assume gcp
	regex := "^[a-z]+-[a-z]+\\d-[a-z]$"
	if Cloud(cloud).IsAWS() {
		regex = "^[a-z]+-[a-z]+-\\d$"
	}
	return &CostAnalyzerProm{
		promEndpoint:       promEndpoint,
		errChan:            make(chan error),
		client:             client,
		localityMatch:      regex,
		attemptsForwarding: 0,
		attemptsThreshold:  1,
	}, nil
}

// PortForwardProm will execute a kubectl port-forward command, forwarding the inbuild prometheus
// deployment to port 9090 on localhost. This is executed asynchronously, and if there is an error,
// it is sent into d.errChan.
func (d *CostAnalyzerProm) PortForwardProm(promNamespace string) {
	cmd := exec.Command("kubectl", "-n", promNamespace, "port-forward", "deployment/prometheus", "9990:9090")
	o, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(o), "address already in use") && d.attemptsForwarding < d.attemptsThreshold {
			fmt.Printf("port-forward failed once, trying again...\n")
			d.attemptsForwarding++
			d.PortForwardProm(promNamespace)
		} else {
			fmt.Printf("cannot port-forward to prometheus: %v %v", err, string(o))
			d.errChan <- err
			return
		}
	}
}

// WaitForProm just pings the prometheus endpoint until it gets a code within [200, 300).
// It selects for that event and an error coming in from d.errChan.
func (d *CostAnalyzerProm) WaitForProm() error {
	ticker := time.NewTicker(500 * time.Millisecond)
	fmt.Println("Waiting for prometheus to be ready...")
	for {
		select {
		case <-ticker.C:
			r, e := http.Get(d.promEndpoint)
			if e == nil {
				fmt.Printf("Prometheus is ready! (Code: %v)\n", r.StatusCode)
				ticker.Stop()
				return nil
			}
		case e := <-d.errChan:
			return e
		}
	}
}

// GetCalls queries the prometheus API for istio_request_bytes_sum, given a time range.
// returns an array of Calls, which contain locality and workload information.
// todo take an actual timerange, and not the hacky "since" parameter.
func (d *CostAnalyzerProm) GetCalls(start, end *time.Time) ([]*Call, error) {
	promApi := v1.NewAPI(d.client)
	calls := make([]*Call, 0)
	query := "istio_request_bytes_sum{destination_locality!=\"\", destination_locality!=\"unknown\"}"
	var result model.Value
	var warn v1.Warnings
	var err error
	if start == nil {
		fmt.Printf("EFN")
		result, warn, err = promApi.Query(context.Background(), query, *end)
		v := result.(model.Vector)
		for i := 0; i < len(v); i++ {
			// check if the locality is valid with regexp, if not, throw it out
			// we do this because anyone can set labels on pods, and we don't want to
			// count those.
			if !d.validateLocality(string(v[i].Metric["destination_locality"])) {
				fmt.Printf("skipping invalid destination locality: %v\n", v[i].Metric["destination_locality"])
				continue
			}
			if !d.validateLocality(string(v[i].Metric["locality"])) {
				fmt.Printf("skipping invalid source locality: %v\n", v[i].Metric["locality"])
				continue
			}
			calls = append(calls, &Call{
				From:         string(v[i].Metric["destination_locality"]),
				To:           string(v[i].Metric["locality"]),
				ToWorkload:   string(v[i].Metric["destination_workload"]),
				FromWorkload: string(v[i].Metric["source_workload"]),
				CallSize:     uint64(v[i].Value),
			})
		}
	} else {
		// query across timerange
		startResult, _, err := promApi.Query(context.Background(), query, *start)
		if err != nil {
			return nil, err
		}
		endResult, _, err := promApi.Query(context.Background(), query, *end)
		if err != nil {
			return nil, err
		}
		startV := startResult.(model.Vector)
		endV := endResult.(model.Vector)
		for i := 0; i < len(startV); i++ {
			if !d.validateLocality(string(startV[i].Metric["destination_locality"])) {
				fmt.Printf("skipping invalid destination locality: %v\n", startV[i].Metric["destination_locality"])
				continue
			}
			if !d.validateLocality(string(startV[i].Metric["locality"])) {
				fmt.Printf("skipping invalid source locality: %v\n", startV[i].Metric["locality"])
				continue
			}
			for j := 0; j < len(endV); j++ {
				if !d.validateLocality(string(endV[j].Metric["destination_locality"])) {
					fmt.Printf("skipping invalid destination locality: %v\n", endV[j].Metric["destination_locality"])
					continue
				}
				if !d.validateLocality(string(endV[j].Metric["locality"])) {
					fmt.Printf("skipping invalid source locality: %v\n", endV[j].Metric["locality"])
					continue
				}
				if string(startV[i].Metric["destination_locality"]) == string(endV[j].Metric["destination_locality"]) &&
					string(startV[i].Metric["locality"]) == string(endV[j].Metric["locality"]) &&
					string(startV[i].Metric["destination_workload"]) == string(endV[j].Metric["destination_workload"]) &&
					string(startV[i].Metric["source_workload"]) == string(endV[j].Metric["source_workload"]) {
					calls = append(calls, &Call{
						From:         string(startV[i].Metric["destination_locality"]),
						To:           string(startV[i].Metric["locality"]),
						ToWorkload:   string(startV[i].Metric["destination_workload"]),
						FromWorkload: string(startV[i].Metric["source_workload"]),
						CallSize:     uint64(endV[j].Value) - uint64(startV[i].Value),
					})
				}
			}
		}
	}
	if err != nil {
		fmt.Printf("error querying prom: %v", err)
		return nil, err
	}
	if len(warn) > 0 {
		fmt.Printf("Warn: %v", warn)
	}

	return calls, nil
}

func (d *CostAnalyzerProm) validateLocality(locality string) bool {
	b, _ := regexp.MatchString(d.localityMatch, locality)
	return b
}
