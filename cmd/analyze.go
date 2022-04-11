package cmd

import (
	"errors"
	"fmt"
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
	details     bool
)

const (
	gcpPricingLocation = "https://raw.githubusercontent.com/tetratelabs/istio-cost-analyzer/master/pricing/gcp_pricing.json"
	awsPricingLocation = "https://github.com/tetratelabs/istio-cost-analyzer/blob/master/pricing/aws/aws_pricing.json"
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
			if cloud == "gcp" {
				pricePath = gcpPricingLocation
			} else if cloud == "aws" {
				pricePath = awsPricingLocation
			} else {
				fmt.Println("when no price path is provided, the only supported clouds are gcp and aws.")
				return errors.New("provide different cloud")
			}
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
		pkg.PrintCostTable(localityCalls, totalCost, details)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cloud, "cloud", "gcp", "aws/gcp/azure are provided by default. if nothing is set, gcp is used.")
	rootCmd.PersistentFlags().StringVar(&pricePath, "pricePath", "", "if custom egress rates are provided, dapani will use the rates in this file.")
	rootCmd.PersistentFlags().StringVar(&queryBefore, "queryBefore", "0s", "if provided a time duration (go format), dapani will only use data from that much time ago and before.")
	rootCmd.PersistentFlags().BoolVar(&details, "details", false, "if true, tool will provide a more detailed view of egress costs, including both destination and source")
	rootCmd.AddCommand(analyzeCmd)
}
