package cmd

import (
	"github.com/spf13/cobra"
	"istio-cost-analyzer/pkg"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"time"
)

const prometheusEndpoint = "http://localhost:9990"

var (
	cloud       string
	pricePath   string
	queryBefore string
)

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
		if pricePath == "" {
			pricePath = "pricing/" + cloud + ".json"
		}
		cost, err := pkg.NewCostAnalysis(pricePath)
		if err != nil {
			return err
		}
		go dapani.PortForwardProm()
		if err := dapani.WaitForProm(); err != nil {
			return err
		}
		duration, err := time.ParseDuration(queryBefore)
		if err != nil {
			return err
		}
		podCalls, err := dapani.GetPodCalls(duration)
		if err != nil {
			return err
		}
		localityCalls, err := kubeClient.GetLocalityCalls(podCalls)
		if err != nil {
			return err
		}
		totalCost, err := cost.CalculateEgress(localityCalls)
		if err != nil {
			return err
		}
		pkg.PrintCostTable(localityCalls, totalCost)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cloud, "cloud", "gcp", "aws/gcp/azure are provided by default. if nothing is set, gcp is used.")
	rootCmd.PersistentFlags().StringVar(&pricePath, "pricePath", "", "if custom egress rates are provided, dapani will use the rates in this file.")
	rootCmd.PersistentFlags().StringVar(&queryBefore, "queryBefore", "0s", "if provided a time duration (go format), dapani will only use data from that much time ago and before.")
	rootCmd.AddCommand(analyzeCmd)
}
