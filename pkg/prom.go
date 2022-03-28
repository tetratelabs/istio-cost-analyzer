package pkg

import (
	"fmt"
	"net/http"
	"os/exec"
	"time"
)

type DapaniProm struct {
	promEndpoint string
	errChan      chan error
}

func NewDapaniProm(promEndpoint string) *DapaniProm {
	return &DapaniProm{
		promEndpoint: promEndpoint,
		errChan:      make(chan error),
	}
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
				fmt.Printf("prometheus is ready! (%v)\n", r.StatusCode)
				ticker.Stop()
				return nil
			}
		case e := <-d.errChan:
			return e
		}
	}
}
