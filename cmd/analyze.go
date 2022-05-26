package cmd

import (
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/tetratelabs/istio-cost-analyzer/pkg"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"time"
)

const prometheusEndpoint = "http://localhost:9990"

var (
	cloud       string
	pricePath   string
	queryBefore string
	details     bool
	namespace   string
)

// todo these should change to tetrate-hosted s3 files, with which we can send over cluster information
// 	to track usage patterns.
const (
	gcpPricingLocation = "https://raw.githubusercontent.com/tetratelabs/istio-cost-analyzer/master/pricing/gcp/gcp_pricing.json"
	awsPricingLocation = "https://raw.githubusercontent.com/tetratelabs/istio-cost-analyzer/master/pricing/aws/aws_pricing.json"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "List all the pod to pod links in the mesh",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		analyzerProm, err := pkg.NewAnalyzerProm(prometheusEndpoint)
		kubeClient := pkg.NewAnalyzerKube()
		if err != nil {
			return err
		}
		if pricePath == "" {
			if cloud == "gcp" {
				pricePath = gcpPricingLocation
			} else if cloud == "aws" {
				pricePath = awsPricingLocation
			} else {
				fmt.Println("when no price path is provided, the only supported clouds are gcp and aws.")
				return errors.New("provide different cloud")
			}
		}
		// initialize analyzer
		cost, err := pkg.NewCostAnalysis(pricePath)
		if err != nil {
			return err
		}
		// port-forward prometheus asynchronously and wait for it to be ready
		go analyzerProm.PortForwardProm()
		if err := analyzerProm.WaitForProm(); err != nil {
			return err
		}
		duration, err := time.ParseDuration(queryBefore)
		if err != nil {
			return err
		}
		// query prometheus for raw pod calls
		podCalls, err := analyzerProm.GetPodCalls(duration)
		if err != nil {
			return err
		}
		// transform raw pod calls to locality information
		localityCalls, err := kubeClient.GetLocalityCalls(podCalls, cloud)
		if err != nil {
			return err
		}
		//localityCalls[0].From = "us-west1-c"
		// calculate egress given locality information
		totalCost, err := cost.CalculateEgress(localityCalls)
		if err != nil {
			return err
		}
		pkg.PrintCostTable(localityCalls, totalCost, details)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cloud, "cloud", "gcp", "aws/gcp/azure are provided by default. if nothing is set, gcp is used.")
	rootCmd.PersistentFlags().StringVar(&pricePath, "pricePath", "", "if custom egress rates are provided, dapani will use the rates in this file.")
	rootCmd.PersistentFlags().StringVar(&queryBefore, "queryBefore", "0s", "if provided a time duration (go format), dapani will only use data from that much time ago and before.")
	rootCmd.PersistentFlags().BoolVar(&details, "details", false, "if true, tool will provide a more detailed view of egress costs, including both destination and source")
	rootCmd.PersistentFlags().StringVar(&namespace, "namespace", "default", "analyze the cost of a certain namespace")
	rootCmd.AddCommand(analyzeCmd)
}
