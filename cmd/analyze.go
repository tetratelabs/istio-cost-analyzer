package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"net/http"
	"os/exec"
	"time"
)

const prometheusEndpoint = "http://localhost:9990"

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "List all the pod to pod links in the mesh",
	Long:  ``,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		status := make(chan error, 0)
		go func(status chan error) {
			cmd := exec.Command("kubectl", "-n", "istio-system", "port-forward", "deployment/prometheus", "9990:9090")
			o, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Printf("cannot port-forward to prometheus: %v %v", err, string(o))
				status <- err
				return
			}
			status <- nil
		}(status)
		fmt.Println("Waiting for prometheus to be ready...")
		for {
			r, e := http.Get(prometheusEndpoint)
			if e == nil {
				fmt.Printf("prometheus is ready! (%v)\n", r.StatusCode)
				break
			} else {
				time.Sleep(1000)
			}
		}
		for {
			
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		// query destination_pod and kubernetes_pod_name

		// get all metrics in istio_request_bytes_sum that have destination_pod

	},
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
}
