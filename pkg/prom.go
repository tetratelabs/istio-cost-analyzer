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
	"time"
)

// CostAnalyzerProm holds the prometheus routines necessary to collect
// service<->service traffic data.
type CostAnalyzerProm struct {
	promEndpoint  string
	errChan       chan error
	client        api.Client
	localityMatch string
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
	if cloud == "aws" {
		regex = "^[a-z]+-[a-z]+-[a-z]\\d$"
	}
	return &CostAnalyzerProm{
		promEndpoint:  promEndpoint,
		errChan:       make(chan error),
		client:        client,
		localityMatch: regex,
	}, nil
}

// PortForwardProm will execute a kubectl port-forward command, forwarding the inbuild prometheus
// deployment to port 9090 on localhost. This is executed asynchronously, and if there is an error,
// it is sent into d.errChan.
func (d *CostAnalyzerProm) PortForwardProm(promNamespace string) {
	cmd := exec.Command("kubectl", "-n", promNamespace, "port-forward", "deployment/prometheus", "9990:9090")
	o, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("cannot port-forward to prometheus: %v %v", err, string(o))
		d.errChan <- err
		return
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
func (d *CostAnalyzerProm) GetCalls(since time.Duration) ([]*Call, error) {
	promApi := v1.NewAPI(d.client)
	calls := make([]*Call, 0)
	query := "istio_request_bytes_sum{destination_locality!=\"\", destination_locality!=\"unknown\"}"
	var result model.Value
	var warn v1.Warnings
	var err error
	if since == 0 {
		result, warn, err = promApi.Query(context.Background(), query, time.Now())
	} else {
		result, warn, err = promApi.Query(context.Background(), query, time.Now().Add(-since))
	}
	if err != nil {
		fmt.Printf("error querying prom: %v", err)
		return nil, err
	}
	if len(warn) > 0 {
		fmt.Printf("Warn: %v", warn)
	}
	v := result.(model.Vector)
	for i := 0; i < len(v); i++ {
		// check if the locality is valid with regexp, if not, throw it out
		// we do this because anyone can set labels on pods, and we don't want to
		// count those.
		if sourceMatch, _ := regexp.MatchString(d.localityMatch, string(v[i].Metric["destination_locality"])); !sourceMatch {
			continue
		}
		if destMatch, _ := regexp.MatchString(d.localityMatch, string(v[i].Metric["locality"])); !destMatch {
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
	return calls, nil
}
