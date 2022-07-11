package cmd

import (
	"bytes"
	"github.com/spf13/cobra"
	"github.com/tetratelabs/istio-cost-analyzer/pkg"
	v1 "k8s.io/api/apps/v1"
	v12 "k8s.io/api/core/v1"
	v13 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8Yaml "k8s.io/apimachinery/pkg/util/yaml"
)

var costAnalyzerSA = &v12.ServiceAccount{
	ObjectMeta: metav1.ObjectMeta{
		Name: "cost-analyzer-sa",
	},
}

var costAnalyzerClusterRoleBinding = &v13.ClusterRoleBinding{
	ObjectMeta: metav1.ObjectMeta{
		Name: "cost-analyzer-role-binding",
	},
	RoleRef: v13.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     "ClusterRole",
		Name:     "cost-analyzer-service-role",
	},
	Subjects: []v13.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      "cost-analyzer-sa",
			Namespace: analyzerNamespace,
		},
	},
}

var costAnalyzerClusterRole = &v13.ClusterRole{
	ObjectMeta: metav1.ObjectMeta{
		Name: "cost-analyzer-service-role",
	},
	Rules: []v13.PolicyRule{
		{
			APIGroups: []string{"", "admissionregistration.k8s.io", "apps"},
			Resources: []string{"mutatingwebhookconfigurations", "pods", "nodes", "deployments"},
			Verbs:     []string{"get", "create", "patch", "list", "update"},
		},
	},
}

var webhookSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Create the webhook object in kubernetes and deploy the server container.",
	Long:  "Setting up a webhook to receive config changes makes it so you don't have to manually change all the configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		kubeClient := pkg.NewAnalyzerKube(kubeconfig)
		//ic := kubeClient.IstioClient()
		var err error
		webhookDeployment := `
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
      containers:
        - name: cost-analyzer-mutating-webhook
          image: adiprerepa/cost-analyzer-mutating-webhook:latest
          imagePullPolicy: Always
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
`
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
		var sa *v12.ServiceAccount
		var cr *v13.ClusterRole
		var crb *v13.ClusterRoleBinding
		var status bool
		cmd.Println("creating webhook deployment/service and role/binding...")
		if sa, err, status = kubeClient.CreateServiceAccount(costAnalyzerSA, analyzerNamespace); err != nil {
			cmd.PrintErrf("unable to create service account: %v", err)
			return err
		} else {
			if status {
				cmd.Printf("service account %v already exists\n", costAnalyzerSA.Name)
			} else {
				cmd.Printf("service account %v created\n", sa.Name)
			}
		}
		if cr, err, status = kubeClient.CreateClusterRole(costAnalyzerClusterRole); err != nil {
			cmd.PrintErrf("unable to create cluster role: %v", err)
			return err
		} else {
			if status {
				cmd.Printf("cluster role %v already exists\n", costAnalyzerClusterRole.Name)
			} else {
				cmd.Printf("cluster role %v created\n", cr.Name)
			}
		}
		// todo dont do this now and actually properly structure the stuff
		costAnalyzerClusterRoleBinding.Subjects[0].Namespace = analyzerNamespace
		if crb, err, status = kubeClient.CreateClusterRoleBinding(costAnalyzerClusterRoleBinding); err != nil {
			cmd.PrintErrf("unable to create cluster role binding: %v", err)
			return err
		} else {
			if status {
				cmd.Printf("cluster role binding %v already exists\n", costAnalyzerClusterRoleBinding.Name)
			} else {
				cmd.Printf("cluster role binding %v created\n", crb.Name)
			}
		}
		depl := &v1.Deployment{}
		decoder := k8Yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(webhookDeployment)), 1000)
		if err = decoder.Decode(&depl); err != nil {
			cmd.PrintErrf("unable to decode deployment: %v", err)
			return err
		}
		depl.Spec.Template.Spec.Containers[0].Env = []v12.EnvVar{{
			Name:  "CLOUD",
			Value: cloud,
		}, {
			Name:  "NAMESPACE",
			Value: targetNamespace,
		}}
		depl.Spec.Template.Spec.InitContainers[0].Env = append(depl.Spec.Template.Spec.InitContainers[0].Env, v12.EnvVar{
			Name:  "WEBHOOK_NAMESPACE",
			Value: analyzerNamespace,
		})
		serv := &v12.Service{}
		decoder = k8Yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(webhookService)), 1000)
		if err = decoder.Decode(&serv); err != nil {
			cmd.PrintErrf("unable to decode service: %v", err)
			return err
		}
		if serv, err, status = kubeClient.CreateService(serv, analyzerNamespace); err != nil {
			cmd.PrintErrf("unable to create service: %v", err)
			return err
		} else {
			if status {
				cmd.Printf("service %v already exists\n", serv.Name)
			} else {
				cmd.Printf("service %v created\n", serv.Name)
			}
		}
		if depl, err, status = kubeClient.CreateDeployment(depl, analyzerNamespace); err != nil {
			cmd.PrintErrf("unable to create deployment: %v", err)
			return err
		} else {
			if status {
				cmd.Printf("deployment %v already exists\n", depl.Name)
			} else {
				cmd.Printf("deployment %v created\n", depl.Name)
			}
		}

		err = kubeClient.CreateIstioOperator("cost-istio-operator", "istio-system")
		//if operatorName == "" {
		//	err := kubeClient.EditIstioOperator("", "")
		//	if err != nil {
		//		cmd.PrintErrf("unable to edit Istio Operator: %v", err)
		//	}
		//}
		if err != nil {
			cmd.PrintErrf("unable to create Istio Operator: %v", err)
		}
		return nil
	},
}
