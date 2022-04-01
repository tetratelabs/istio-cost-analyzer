package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "istio-cost-analyzer",
	Short: "Istio Cost Tooling",

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("run istio-cost-analyzer analyze")
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
