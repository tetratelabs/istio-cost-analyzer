package pkg

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"net/http"
	"os/exec"
	"time"
)

type DapaniProm struct {
	promEndpoint string
	errChan      chan error
	client       api.Client
}

func NewDapaniProm(promEndpoint string) (*DapaniProm, error) {
	client, err := api.NewClient(api.Config{
		Address: promEndpoint,
	})
	if err != nil {
		fmt.Printf("cannot initialize prom lib: %v", err)
		return nil, err
	}
	return &DapaniProm{
		promEndpoint: promEndpoint,
		errChan:      make(chan error),
		client:       client,
	}, nil
}

func (d *DapaniProm) PortForwardProm() {
	cmd := exec.Command("kubectl", "-n", "istio-system", "port-forward", "deployment/prometheus", "9990:9090")
	o, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("cannot port-forward to prometheus: %v %v", err, string(o))
		d.errChan <- err
		return
	}
}

func (d *DapaniProm) WaitForProm() error {
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
func (d *DapaniProm) GetPodCalls(since time.Duration) ([]*PodCall, error) {
	promApi := v1.NewAPI(d.client)
	calls := make([]*PodCall, 0)
	query := "istio_request_bytes_sum{destination_pod!=\"\", destination_pod!=\"unknown\"}"
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
		// todo collapse pod<->pod into workload<->workload
		calls = append(calls, &PodCall{
			ToPod:        string(v[i].Metric["destination_pod"]),
			FromPod:      string(v[i].Metric["kubernetes_pod_name"]),
			ToWorkload:   string(v[i].Metric["destination_workload"]),
			FromWorkload: string(v[i].Metric["source_workload"]),
			CallSize:     uint64(v[i].Value),
		})
	}
	return calls, nil
}
