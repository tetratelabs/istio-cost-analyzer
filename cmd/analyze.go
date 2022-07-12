package cmd

import (
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/tetratelabs/istio-cost-analyzer/pkg"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/util/homedir"
	"os"
	"path/filepath"
	"time"
)

const prometheusEndpoint = "http://localhost:9990"

var (
	cloud             string
	pricePath         string
	queryBefore       string
	details           bool
	promNs            string
	analyzerNamespace string
	targetNamespace   string
	operatorName      string
	operatorNamespace string
	kubeconfig        string
)

// todo these should change to tetrate-hosted s3 files, with which we can send over cluster information
// 	to track usage patterns.
const (
	gcpPricingLocation = "https://raw.githubusercontent.com/tetratelabs/istio-cost-analyzer/master/pricing/gcp/gcp_pricing.json"
	awsPricingLocation = "https://raw.githubusercontent.com/tetratelabs/istio-cost-analyzer/master/pricing/aws/aws_pricing.json"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "List all the service links in the mesh",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		analyzerProm, err := pkg.NewAnalyzerProm(prometheusEndpoint, cloud)
		kubeClient := pkg.NewAnalyzerKube(kubeconfig)
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
		go analyzerProm.PortForwardProm(promNs)
		if err := analyzerProm.WaitForProm(); err != nil {
			return err
		}
		duration, err := time.ParseDuration(queryBefore)
		if err != nil {
			return err
		}
		// query prometheus for raw pod calls
		localityCalls, err := analyzerProm.GetCalls(duration)
		if err != nil {
			return err
		}
		// transform raw pod calls to locality information
		localityCalls, err = kubeClient.TransformLocalityCalls(localityCalls)
		if err != nil {
			return err
		}
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
	defaultKube := ""
	if os.Getenv("KUBECONFIG") == "" {
		home := homedir.HomeDir()
		defaultKube = filepath.Join(home, ".kube", "config")
	} else {
		defaultKube = os.Getenv("KUBECONFIG")
	}
	// setup/destroy need this
	rootCmd.PersistentFlags().StringVar(&operatorName, "operatorName", "", "name of your istio operator. If not set, cost tool will use the first operator found in the istio-system namespace")
	rootCmd.PersistentFlags().StringVar(&operatorNamespace, "operatorNamespace", "istio-system", "namespace of your istio operator")

	analyzeCmd.PersistentFlags().StringVar(&pricePath, "pricePath", "", "if custom egress rates are provided, dapani will use the rates in this file.")
	analyzeCmd.PersistentFlags().StringVar(&queryBefore, "queryBefore", "0s", "if provided a time duration (go format), dapani will only use data from that much time ago and before.")
	analyzeCmd.PersistentFlags().BoolVar(&details, "details", false, "if true, tool will provide a more detailed view of egress costs, including both destination and source")
	analyzeCmd.PersistentFlags().StringVar(&promNs, "promNamespace", analyzerNamespace, "promNs that the prometheus pod lives in, if different from analyzerNamespace")

	rootCmd.PersistentFlags().StringVar(&cloud, "cloud", "gcp", "aws/gcp/azure are provided by default. if nothing is set, gcp is used.")
	rootCmd.PersistentFlags().StringVar(&analyzerNamespace, "analyzerNamespace", "istio-system", "namespace that the cost analyzer and associated resources lives in")
	rootCmd.PersistentFlags().StringVar(&targetNamespace, "targetNamespace", "default", "namespace that the cost analyzer will analyze")
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", defaultKube, "path to kubeconfig file")

	destroyCmd.PersistentFlags().BoolVarP(&destroyOperator, "destroyOperator", "o", false, "if true, cost analyzer will destroy the istio operator config that it created")

	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(webhookSetupCmd)
	rootCmd.AddCommand(destroyCmd)
}
