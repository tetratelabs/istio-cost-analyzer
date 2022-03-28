package cmd

import (
	"dapani/pkg"
	"github.com/spf13/cobra"
)

const prometheusEndpoint = "http://localhost:9990"

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "List all the pod to pod links in the mesh",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		prom := pkg.NewDapaniProm(prometheusEndpoint)
		go prom.PortForwardProm()
		if err := prom.WaitForProm(); err != nil {
			return err
		}
		return nil
		// get all metrics in istio_request_bytes_sum that have destination_pod
		// get the pods info from client-go
		// get the associated nodes

	},
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
}
