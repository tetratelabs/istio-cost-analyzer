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
	"context"
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tetratelabs/istio-cost-analyzer/pkg"
)

var destroyOperator bool

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy the webhook object in kubernetes and delete the server container.",
	Long:  "Destroying the webhook object in kubernetes and deleting the server container makes it so you don't have to manually change all the configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		kubeClient := pkg.NewAnalyzerKube(kubeconfig)
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
		if destroyOperator {
			if operatorName == "" {
				var err error
				operatorName, err = kubeClient.GetDefaultOperator(operatorNamespace)
				if err != nil {
					fmt.Printf("not destroying operator: %v", err)
					return nil
				}
			}
			if err := kubeClient.DeleteOperatorConfig(operatorName, operatorNamespace); err != nil {
				fmt.Println(err)
			}
		}
		return nil
	},
}
