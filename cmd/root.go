package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "dapani",
	Short: "Istio Cost Tooling",

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("yo")
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {

	/*
			flags for:
			- kubeconfig location
				default is filepath.Join(home, ".kube", "config"), home=homedir.HomeDir()
		use client-go
		use prometheus to get kubernetes_pod_name and destination_pod
		query nodes of kubernetes_pod_name and destination_pod
		query regions/localities of nodes of kubernetes_pod_name and destination_pod
		apply cross-region
	*/

	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
