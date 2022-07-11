package pkg

import (
	"context"
	"fmt"
	"istio.io/client-go/pkg/clientset/versioned"
	v12 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	v13 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"strings"
)

var iopResource = schema.GroupVersionResource{Group: "install.istio.io", Version: "v1alpha1", Resource: "istiooperators"}

// KubeClient just wraps the kubernetes API.
// todo should we just do:
//  ```
//   type KubeClient kubernetes.ClientSet
//  ```
// if we get no value from just wrapping?
type KubeClient struct {
	clientSet  *kubernetes.Clientset
	dynamic    dynamic.Interface
	kubeconfig string
}

// NewAnalyzerKube creates a clientset using the kubeconfig found in the home directory.
// todo make kubeconfig a settable parameter in analyzer.go
func NewAnalyzerKube(kubeconfig string) *KubeClient {
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	dynamicClient := dynamic.NewForConfigOrDie(config)
	return &KubeClient{
		clientSet:  clientset,
		kubeconfig: kubeconfig,
		dynamic:    dynamicClient,
	}
}

// TransformLocalityCalls takes a raw list of type Call and collapses the data
// into a per-link basis (there might be multiple metrics for locality a->b)
// todo maybe do this directly in prom.go and make it O(n) instead of O(2n)
// sort of legacy?
func (k *KubeClient) TransformLocalityCalls(rawCalls []*Call) ([]*Call, error) {
	calls := make([]*Call, 0)
	// serviceCallMap's keys are just workload/locality links, without any call size information,
	// while the map value is the full, aggregated call value for that link. We do this because there may
	// exist multiple pods that cause the same workload/locality link, and we don't want them to duplicate.
	serviceCallMap := make(map[Call]*Call)
	for i := 0; i < len(rawCalls); i++ {
		serviceLocalityKey := Call{
			FromWorkload: rawCalls[i].FromWorkload,
			From:         rawCalls[i].From,
			ToWorkload:   rawCalls[i].ToWorkload,
			To:           rawCalls[i].To,
		}
		// either create a new entry, or add to an existing one.
		if _, ok := serviceCallMap[serviceLocalityKey]; !ok {
			serviceCallMap[serviceLocalityKey] = &serviceLocalityKey
			serviceLocalityKey.CallSize = rawCalls[i].CallSize
		} else {
			serviceCallMap[serviceLocalityKey].CallSize += rawCalls[i].CallSize
		}
		if i%10 == 0 {
			for k, v := range serviceCallMap {
				fmt.Printf("%v(%v) -> %v(%v): %v  |  link %v / %v\n", k.From, k.FromWorkload, k.To, k.ToWorkload, v.CallSize, i, len(rawCalls))
			}
		}
	}
	for _, v := range serviceCallMap {
		calls = append(calls, v)
	}
	return calls, nil
}

// getPodNode gets the node associated with a given pod name in the default namespece.
func (k *KubeClient) getPodNode(name, namespace string) (string, error) {
	pod, err := k.clientSet.CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("error in getting pod %v: %v\n", name, err)
		return "", err
	}
	return pod.Spec.NodeName, nil
}

// getNodeLocality gets the locality given by topology.kubernetes.io.
func (k *KubeClient) getNodeLocality(name, cloud string) (string, error) {
	// if we are on AWS, we want to just get region, because availability zones
	// are not supported yet.
	if cloud == "aws" {
		return k.getNodeLabel(name, "topology.kubernetes.io/region")
	}
	return k.getNodeLabel(name, "topology.kubernetes.io/zone")
}

func (k *KubeClient) getNodeLabel(name, label string) (string, error) {
	node, err := k.clientSet.CoreV1().Nodes().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("error in getting node %v: %v\n", name, err)
		return "", err
	}
	return node.Labels[label], nil
}

// CreateService creates a service in the given namespace. Returns the service, the error, and
// a boolean representing whether or not the service already exists.
func (k *KubeClient) CreateService(service *v1.Service, ns string) (*v1.Service, error, bool) {
	svc, err := k.clientSet.CoreV1().Services(ns).Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil && strings.Contains(err.Error(), "already exists") {
		return service, nil, true
	}
	return svc, err, false
}

func (k *KubeClient) CreateDeployment(deployment *v12.Deployment, ns string) (*v12.Deployment, error, bool) {
	dep, err := k.clientSet.AppsV1().Deployments(ns).Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil && strings.Contains(err.Error(), "already exists") {
		return deployment, nil, true
	}
	return dep, err, false
}

func (k *KubeClient) CreateServiceAccount(serviceAccount *v1.ServiceAccount, ns string) (*v1.ServiceAccount, error, bool) {
	sa, err := k.clientSet.CoreV1().ServiceAccounts(ns).Create(context.TODO(), serviceAccount, metav1.CreateOptions{})
	if err != nil && strings.Contains(err.Error(), "already exists") {
		return serviceAccount, nil, true
	}
	return sa, err, false
}

func (k *KubeClient) CreateClusterRoleBinding(clusterRoleBinding *v13.ClusterRoleBinding) (*v13.ClusterRoleBinding, error, bool) {
	crb, err := k.clientSet.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, metav1.CreateOptions{})
	if err != nil && strings.Contains(err.Error(), "already exists") {
		return clusterRoleBinding, nil, true
	}
	return crb, err, false
}

func (k *KubeClient) CreateClusterRole(clusterRole *v13.ClusterRole) (*v13.ClusterRole, error, bool) {
	cr, err := k.clientSet.RbacV1().ClusterRoles().Create(context.TODO(), clusterRole, metav1.CreateOptions{})
	if err != nil && strings.Contains(err.Error(), "already exists") {
		return clusterRole, nil, true
	}
	return cr, err, false
}

func (k *KubeClient) Client() kubernetes.Interface {
	return k.clientSet
}

func (k *KubeClient) IstioClient() *versioned.Clientset {
	config, err := clientcmd.BuildConfigFromFlags("", k.kubeconfig)
	if err != nil {
		panic(err.Error())
	}
	return versioned.NewForConfigOrDie(config)
}

func (k *KubeClient) CreateIstioOperator(opName, opNamespace string) error {
	istioOperator := &unstructured.Unstructured{}
	istioOperator.SetUnstructuredContent(map[string]interface{}{
		"apiVersion": "install.istio.io/v1alpha1",
		"kind":       "IstioOperator",
		"metadata": map[string]interface{}{
			"name":      opName,
			"namespace": opNamespace,
		},
		"spec": map[string]interface{}{
			"profile": "demo",
			//"values": map[string]interface{}{
			//	"telemetry": map[string]interface{}{
			//		"prometheus": map[string]interface{}{
			//			"configOverride": map[string]interface{}{
			//				"inboundSidecar": map[string]interface{}{
			//					"metrics": []interface{}{
			//						map[string]interface{}{
			//							"name": "request_bytes",
			//							"dimensions": map[string]interface{}{
			//								"destination_locality": "downstream_peer.labels['locality'].value",
			//							},
			//						},
			//					},
			//				},
			//				"outboundSidecar": map[string]interface{}{
			//					"metrics": []interface{}{
			//						map[string]interface{}{
			//							"name": "request_bytes",
			//							"dimensions": map[string]interface{}{
			//								"destination_locality": "upstream_peer.labels['locality'].value",
			//							},
			//						},
			//					},
			//				},
			//			},
			//		},
			//	},
			//},
		},
	})
	_, err := k.dynamic.Resource(iopResource).Namespace(opNamespace).Create(context.TODO(), istioOperator, metav1.CreateOptions{})
	return err
}

func (k *KubeClient) EditIstioOperator(opName, opNamespace string) error {
	_, err := k.dynamic.Resource(iopResource).Namespace(opNamespace).Get(context.TODO(), opName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	patch := `
[
   {
      "path":"/spec",
      "op":"add",
      "value":{
         "values":{
            "telemetry":{
               "v2":{
                  "prometheus":{
                     "configOverride":{
                        "inboundSidecar":{
                           "metrics":[
                              {
                                 "dimensions":{
                                    "destination_locality":"downstream_peer.labels['locality'].value"
                                 },
                                 "name":"request_bytes"
                              }
                           ]
                        },
                        "outboundSidecar":{
                           "metrics":[
                              {
                                 "dimensions":{
                                    "destination_locality":"upstream_peer.labels['locality'].value"
                                 },
                                 "name":"request_bytes"
                              }
                           ]
                        }
                     }
                  }
               }
            }
         }
      }
   }
]
`
	_, err = k.dynamic.Resource(iopResource).Namespace(opNamespace).Patch(context.TODO(), opName, types.JSONPatchType, []byte(patch), metav1.PatchOptions{})
	if err != nil {
		return err
	}
	//jsonStr, err := json.Marshal(res)
	//if err != nil {
	//	return err
	//}
	//fmt.Printf("OPERATOR: %v\n", string(jsonStr))
	//var iop *unstructured.Unstructured
	//for _, item := range list.Items {
	//	if item.GetName() == opName {
	//		iop = &item
	//		break
	//	}
	//}
	return nil
}
