package cmd

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/tetratelabs/istio-cost-analyzer/pkg"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy the webhook object in kubernetes and delete the server container.",
	Long:  "Destroying the webhook object in kubernetes and deleting the server container makes it so you don't have to manually change all the configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		kubeClient := pkg.NewAnalyzerKube()
		// todo make config names package-wide constants
		// todo more visibility into errors
		if err := kubeClient.Client().AppsV1().Deployments(analyzerNamespace).Delete(context.TODO(), "cost-analyzer-mutating-webhook", metav1.DeleteOptions{}); err != nil {
			fmt.Println(err)
		}
		if err := kubeClient.Client().CoreV1().Services(analyzerNamespace).Delete(context.TODO(), "cost-analyzer-mutating-webhook", metav1.DeleteOptions{}); err != nil {
			fmt.Println(err)
		}
		if err := kubeClient.Client().AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(context.TODO(), "cost-analyzer-mutating-webhook-configuration", metav1.DeleteOptions{}); err != nil {
			fmt.Println(err)
		}
		if err := kubeClient.Client().RbacV1().ClusterRoleBindings().Delete(context.TODO(), "cost-analyzer-role-binding", metav1.DeleteOptions{}); err != nil {
			fmt.Println(err)
		}
		if err := kubeClient.Client().RbacV1().ClusterRoles().Delete(context.TODO(), "cost-analyzer-service-role", metav1.DeleteOptions{}); err != nil {
			fmt.Println(err)
		}
		if err := kubeClient.Client().CoreV1().ServiceAccounts(analyzerNamespace).Delete(context.TODO(), "cost-analyzer-sa", metav1.DeleteOptions{}); err != nil {
			fmt.Println(err)
		}
		return nil
	},
}