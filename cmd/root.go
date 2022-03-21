/*
Copyright © 2022 Tetrate

*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "dapani",
	Short: "Istio cost tooling",
	Long:  ``,
	Run:   func(cmd *cobra.Command, args []string) {},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.dapani.yaml)")

	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
