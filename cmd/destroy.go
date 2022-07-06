package cmd

import (
	"context"
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
		_ = kubeClient.Client().AppsV1().Deployments(promNs).Delete(context.TODO(), "cost-analyzer-mutating-webhook", metav1.DeleteOptions{})
		_ = kubeClient.Client().CoreV1().Services(promNs).Delete(context.TODO(), "cost-analyzer-mutating-webhook", metav1.DeleteOptions{})
		_ = kubeClient.Client().AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(context.TODO(), "cost-analyzer-mutating-webhook-configuration", metav1.DeleteOptions{})
		_ = kubeClient.Client().RbacV1().ClusterRoleBindings().Delete(context.TODO(), "cost-analyzer-role-binding", metav1.DeleteOptions{})
		_ = kubeClient.Client().RbacV1().ClusterRoles().Delete(context.TODO(), "cost-analyzer-service-role", metav1.DeleteOptions{})
		_ = kubeClient.Client().CoreV1().ServiceAccounts(promNs).Delete(context.TODO(), "cost-analyzer-sa", metav1.DeleteOptions{})
		return nil
	},
}
