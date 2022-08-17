// Copyright 2022 Tetrate
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/util/homedir"

	"github.com/tetratelabs/istio-cost-analyzer/pkg"
	"strconv"
)

const prometheusEndpoint = "http://localhost:9990"

var (
	cloud             string
	pricePath         string
	queryBefore       string
	start             string
	end               string
	details           bool
	promNs            string
	analyzerNamespace string
	targetNamespace   string
	analyzeAll        bool
	operatorName      string
	operatorNamespace string
	kubeconfig        string
	verb		  bool
)

// todo these should change to tetrate-hosted s3 files, with which we can send over cluster information
// 	to track usage patterns.
const (
	gcpPricingLocation = "https://raw.githubusercontent.com/tetratelabs/istio-cost-analyzer/master/pricing/gcp/gcp_pricing.json"
	awsPricingLocation = "https://raw.githubusercontent.com/tetratelabs/istio-cost-analyzer/master/pricing/aws/aws_pricing.json"
)

func printf1(statement string, len int, arr []) {
	//useVerb := analyzeCmd.PersistentFlags().Bool(&verb, "v", false, "if true verbose output mode is enabled")
	//flag.Parse()
	if *useverb {
		if len == 0 {
			fmt.println(statement)
		}
		if len == 1 {
			fmt.printf(statement, arr[0])
		}
	}
}
var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "List all the service links in the mesh",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		kubeClient := pkg.NewAnalyzerKube(kubeconfig)
		// if a custom price path isn't provided, use the default price path for the cloud
		// the cluster is on.
		if pricePath == "" {
			if cloud == "" {
				cloud = string(kubeClient.InferCloud())
			}
			cloud = strings.ToUpper(cloud)
			if pkg.Cloud(cloud).IsGCP() {
				pricePath = gcpPricingLocation
			} else if pkg.Cloud(cloud).IsAWS() {
				pricePath = awsPricingLocation
			} else {
				// we don't have a price path or cloud, so fail
				fmt.Println("when no price path is provided, the only supported clouds are gcp and aws. couldn't infer cloud info.")
				printf1("when no price path is provided, the only supported clouds are gcp and aws. couldn't infer cloud info.", 0, [])
				return errors.New("provide different cloud")
			}
			fmt.Printf("found cloud: %s\n", cloud)
			printf1("found cloud: %s\n", 1, [string(cloud)]
		}
		fmt.Printf("using pricing file: %s\n", pricePath)
		printf1("using pricing file: %s\n", 1, [string(pricePath])
		analyzerProm, err := pkg.NewAnalyzerProm(prometheusEndpoint, cloud)
		if err != nil {
			return err
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
		var endTime time.Time
		var startTime *time.Time
		if end == "" {
			endTime = time.Now()
		} else {
			endTime, err = time.Parse(time.RFC3339, end)
			if err != nil {
				return err
			}
		}
		st, err := time.Parse(time.RFC3339, start)
		if start == "" {
			startTime = nil
		} else {
			if err != nil {
				return err
			}
			startTime = &st
		}
		// query prometheus for raw pod calls
		localityCalls, err := analyzerProm.GetCalls(startTime, &endTime)
		if err != nil {
			return err
		}
		// transform raw pod calls to locality information
		localityCalls, err = kubeClient.CollapseLocalityCalls(localityCalls)
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
	analyzeCmd.PersistentFlags().StringVar(&promNs, "prometheusNamespace", "istio-system", "promNs that the prometheus pod lives in, if different from analyzerNamespace")
	analyzeCmd.PersistentFlags().StringVar(&start, "start", "", "if provided, the cost analyzer will analyze costs from this time onwards")
	analyzeCmd.PersistentFlags().StringVar(&end, "end", "", "if provided, the cost analyzer will analyze costs up to this time")
	useverb := analyzeCmd.PersistentFlags().BoolVar(&verb, "v", false, "if true verbose output mode is enabled")

	rootCmd.PersistentFlags().StringVar(&cloud, "cloud", "", "aws/gcp/azure are provided by default. if nothing is set, cloud info is inferred.")
	rootCmd.PersistentFlags().StringVar(&analyzerNamespace, "analyzerNamespace", "istio-system", "namespace that the cost analyzer and associated resources lives in")
	webhookSetupCmd.PersistentFlags().StringVar(&targetNamespace, "targetNamespace", "default", "namespace that the cost analyzer will analyze")
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", defaultKube, "path to kubeconfig file")

	destroyCmd.PersistentFlags().BoolVarP(&destroyOperator, "destroyOperator", "o", false, "if true, cost analyzer will destroy the istio operator config that it created")
	webhookSetupCmd.PersistentFlags().BoolVarP(&analyzeAll, "analyzeAll", "a", false, "if true, cost analyzer will analyze all namespaces in the cluster")

	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(webhookSetupCmd)
	rootCmd.AddCommand(destroyCmd)
}
