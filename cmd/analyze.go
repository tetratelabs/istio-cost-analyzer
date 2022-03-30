package cmd

import (
	"dapani/pkg"
	"fmt"
	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

const prometheusEndpoint = "http://localhost:9990"

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "List all the pod to pod links in the mesh",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		dapani, err := pkg.NewDapaniProm(prometheusEndpoint)
		kubeClient := pkg.NewDapaniKubeClient()
		if err != nil {
			return err
		}
		go dapani.PortForwardProm()
		if err := dapani.WaitForProm(); err != nil {
			return err
		}
		podCalls, err := dapani.GetPodCalls()
		if err != nil {
			return err
		}
		localityCalls, err := kubeClient.GetLocalityCalls(podCalls)
		if err != nil {
			return err
		}
		for _, l := range localityCalls {
			fmt.Printf("%v\n", l.String())
		}
		for {
		}
		return nil
		// egress from -> to

		// https://cloud.google.com/vpc/network-pricing
	},
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
}
