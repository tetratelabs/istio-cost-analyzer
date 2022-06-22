package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/tetratelabs/istio-cost-analyzer/pkg"
	v1 "k8s.io/api/apps/v1"
	v12 "k8s.io/api/core/v1"
	k8Yaml "k8s.io/apimachinery/pkg/util/yaml"
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
		go analyzerProm.PortForwardProm(namespace)
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
		localityCalls[0].From = "us-west1-c"
		// calculate egress given locality information
		totalCost, err := cost.CalculateEgress(localityCalls)
		if err != nil {
			return err
		}
		pkg.PrintCostTable(localityCalls, totalCost, details)
		return nil
	},
}

var webhookSetupCmd = &cobra.Command{
	Use:   "setupWebhook",
	Short: "Create the webhook object in kubernetes and deploy the server container.",
	Long:  "Setting up a webhook to receive config changes makes it so you don't have to manually change all the configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		kubeClient := pkg.NewAnalyzerKube()
		var err error
		webhookDeployment := fmt.Sprintf(`
kind: Deployment
apiVersion: apps/v1
metadata:
  name: cost-analyzer-mutating-webhook
spec:
  replicas: 1
  selector:
    matchLabels:
      app: cost-analyzer-mutating-webhook
  template:
    metadata:
      labels:
        app: cost-analyzer-mutating-webhook
    spec:
      initContainers:
        - name: cost-analyzer-mutating-webhook-ca
          image: adiprerepa/cost-analyzer-mutating-webhook-ca:latest
          imagePullPolicy: Always
          volumeMounts:
            - mountPath: /etc/webhook/certs
              name: certs
          env:
            - name: MUTATE_CONFIG
              value: cost-analyzer-mutating-webhook-configuration
            - name: WEBHOOK_SERVICE
              value: cost-analyzer-mutating-webhook
            - name: WEBHOOK_NAMESPACE
              value: default
      containers:
        - name: cost-analyzer-mutating-webhook
          image: adiprerepa/cost-analyzer-mutating-webhook:latest
          imagePullPolicy: Always
	      env:
			- name: CLOUD
			  value: %v
			- name: NAMESPACE
			  value: %v
          ports:
            - containerPort: 443
          volumeMounts:
            - name: certs
              mountPath: /etc/webhook/certs
          resources: 
            requests:
              memory: "64Mi"
              cpu: "250m"
            limits:
              memory: "128Mi"
              cpu: "500m"
      volumes:
        - name: certs
          emptyDir: {}
      serviceAccountName: cost-analyzer-sa
`, cloud, namespace)
		webhookService := `
kind: Service
apiVersion: v1
metadata:
  name: cost-analyzer-mutating-webhook
spec:
  selector:
    app: cost-analyzer-mutating-webhook
  ports:
    - port: 443
      protocol: TCP
      targetPort: 443
`
		depl := &v1.Deployment{}
		decoder := k8Yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(webhookDeployment)), 1000)
		if err = decoder.Decode(&depl); err != nil {
			cmd.PrintErrf("unable to decode deployment: %v", err)
			return err
		}
		serv := &v12.Service{}
		decoder = k8Yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(webhookService)), 1000)
		if err = decoder.Decode(&serv); err != nil {
			cmd.PrintErrf("unable to decode service: %v", err)
			return err
		}
		if serv, err = kubeClient.CreateService(serv); err != nil {
			cmd.PrintErrf("unable to create service: %v", err)
			return err
		}
		if depl, err = kubeClient.CreateDeployment(depl); err != nil {
			cmd.PrintErrf("unable to create deployment: %v", err)
			return err
		}
		cmd.Printf("successfully created webhook service %v and deployment %v", serv.Name, depl.Name)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cloud, "cloud", "gcp", "aws/gcp/azure are provided by default. if nothing is set, gcp is used.")
	rootCmd.PersistentFlags().StringVar(&pricePath, "pricePath", "", "if custom egress rates are provided, dapani will use the rates in this file.")
	rootCmd.PersistentFlags().StringVar(&queryBefore, "queryBefore", "0s", "if provided a time duration (go format), dapani will only use data from that much time ago and before.")
	rootCmd.PersistentFlags().BoolVar(&details, "details", false, "if true, tool will provide a more detailed view of egress costs, including both destination and source")
	rootCmd.PersistentFlags().StringVar(&namespace, "promNamespace", "istio-system", "namespace that the prometheus pod lives in")
	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(webhookSetupCmd)
}
