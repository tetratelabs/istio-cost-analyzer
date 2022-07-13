package pkg

import (
	"context"
	"errors"
	"fmt"
	"istio.io/client-go/pkg/clientset/versioned"
	v12 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	v13 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func (k *KubeClient) GetDefaultOperator(ns string) (string, error) {
	rl, err := k.dynamic.Resource(iopResource).Namespace(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	for _, r := range rl.Items {
		if _, ok := r.Object["status"]; ok {
			if status, ok := r.Object["status"].(map[string]interface{})["status"]; ok && status != nil && status.(string) == "HEALTHY" {
				return r.GetName(), nil
			}
		}
	}
	return "", errors.New("no default operator found, please specify a healthy istio operator")
}

func (k *KubeClient) EditIstioOperator(opName, opNamespace string) error {
	res, err := k.dynamic.Resource(iopResource).Namespace(opNamespace).Get(context.TODO(), opName, metav1.GetOptions{})
	res, neededUpdate := normalizeOperator(res)
	if err != nil {
		return err
	}
	if !neededUpdate {
		return nil
	}
	_, err = k.dynamic.Resource(iopResource).Namespace(opNamespace).Update(context.TODO(), res, metav1.UpdateOptions{})
	return err
}

func (k *KubeClient) DeleteOperatorConfig(opName, opNs string) error {
	res, err := k.dynamic.Resource(iopResource).Namespace(opNs).Get(context.TODO(), opName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	res = denormalizeOperator(res)
	_, err = k.dynamic.Resource(iopResource).Namespace(opNs).Update(context.TODO(), res, metav1.UpdateOptions{})
	return err
}

// literal trash but it works
func normalizeOperator(res *unstructured.Unstructured) (*unstructured.Unstructured, bool) {
	// lord forgive me for I have sinned
	// theres probably a better way to figure this out with JSONPatch but i'm lazy
	// todo you fucking forgot telemetry v2 proemetheus
	if v := res.Object["spec"].(map[string]interface{})["values"]; v == nil {
		res.Object["spec"].(map[string]interface{})["values"] = make(map[string]interface{})
	}
	if v := res.Object["spec"].(map[string]interface{})["values"].(map[string]interface{})["telemetry"]; v == nil {
		res.Object["spec"].(map[string]interface{})["values"].(map[string]interface{})["telemetry"] = make(map[string]interface{})
	}
	if v := res.Object["spec"].(map[string]interface{})["values"].(map[string]interface{})["telemetry"].(map[string]interface{})["v2"]; v == nil {
		res.Object["spec"].(map[string]interface{})["values"].(map[string]interface{})["telemetry"].(map[string]interface{})["v2"] = make(map[string]interface{})
	}
	if v := res.Object["spec"].(map[string]interface{})["values"].(map[string]interface{})["telemetry"].(map[string]interface{})["v2"].(map[string]interface{})["prometheus"]; v == nil {
		res.Object["spec"].(map[string]interface{})["values"].(map[string]interface{})["telemetry"].(map[string]interface{})["v2"].(map[string]interface{})["prometheus"] = make(map[string]interface{})
	}
	if v := res.Object["spec"].(map[string]interface{})["values"].(map[string]interface{})["telemetry"].(map[string]interface{})["v2"].(map[string]interface{})["prometheus"].(map[string]interface{})["configOverride"]; v == nil {
		res.Object["spec"].(map[string]interface{})["values"].(map[string]interface{})["telemetry"].(map[string]interface{})["v2"].(map[string]interface{})["prometheus"].(map[string]interface{})["configOverride"] = make(map[string]interface{})
	}
	if v := res.Object["spec"].(map[string]interface{})["values"].(map[string]interface{})["telemetry"].(map[string]interface{})["v2"].(map[string]interface{})["prometheus"].(map[string]interface{})["configOverride"].(map[string]interface{})["inboundSidecar"]; v == nil {
		res.Object["spec"].(map[string]interface{})["values"].(map[string]interface{})["telemetry"].(map[string]interface{})["v2"].(map[string]interface{})["prometheus"].(map[string]interface{})["configOverride"].(map[string]interface{})["inboundSidecar"] = make(map[string]interface{})
	}
	inbound := res.Object["spec"].(map[string]interface{})["values"].(map[string]interface{})["telemetry"].(map[string]interface{})["v2"].(map[string]interface{})["prometheus"].(map[string]interface{})["configOverride"].(map[string]interface{})["inboundSidecar"].(map[string]interface{})["metrics"]
	if inbound == nil {
		inbound = make([]interface{}, 0)
	}
	if v := res.Object["spec"].(map[string]interface{})["values"].(map[string]interface{})["telemetry"].(map[string]interface{})["v2"].(map[string]interface{})["prometheus"].(map[string]interface{})["configOverride"].(map[string]interface{})["outboundSidecar"]; v == nil {
		res.Object["spec"].(map[string]interface{})["values"].(map[string]interface{})["telemetry"].(map[string]interface{})["v2"].(map[string]interface{})["prometheus"].(map[string]interface{})["configOverride"].(map[string]interface{})["outboundSidecar"] = make(map[string]interface{})
	}
	outbound := res.Object["spec"].(map[string]interface{})["values"].(map[string]interface{})["telemetry"].(map[string]interface{})["v2"].(map[string]interface{})["prometheus"].(map[string]interface{})["configOverride"].(map[string]interface{})["outboundSidecar"].(map[string]interface{})["metrics"]
	if outbound == nil {
		outbound = make([]interface{}, 0)
	}

	// check if we already wrote
	for _, i := range inbound.([]interface{}) {
		if i.(map[string]interface{})["dimensions"] != nil && i.(map[string]interface{})["dimensions"].(map[string]interface{})["destination_locality"] != nil {
			return res, false
		}
	}
	for _, o := range outbound.([]interface{}) {
		if o.(map[string]interface{})["dimensions"] != nil && o.(map[string]interface{})["dimensions"].(map[string]interface{})["destination_locality"] != nil {
			return res, false
		}
	}

	// actual config
	// we dont want to override anything
	for i, in := range inbound.([]interface{}) {
		if in.(map[string]interface{})["name"].(string) == "request_bytes" {
			in.(map[string]interface{})["dimensions"].(map[string]interface{})["destination_locality"] = "downstream_peer.labels['locality'].value"
		}
		inbound.([]interface{})[i] = in
	}

	for i, out := range outbound.([]interface{}) {
		if out.(map[string]interface{})["name"].(string) == "request_bytes" {
			out.(map[string]interface{})["dimensions"].(map[string]interface{})["destination_locality"] = "upstream_peer.labels['locality'].value"
		}
		outbound.([]interface{})[i] = out
	}

	if len(inbound.([]interface{})) == 0 {
		inbound = append(inbound.([]interface{}), map[string]interface{}{
			"name": "request_bytes",
			"dimensions": map[string]interface{}{
				"destination_locality": "downstream_peer.labels['locality'].value",
			},
		})
	}

	if len(outbound.([]interface{})) == 0 {
		outbound = append(outbound.([]interface{}), map[string]interface{}{
			"name": "request_bytes",
			"dimensions": map[string]interface{}{
				"destination_locality": "upstream_peer.labels['locality'].value",
			},
		})
	}

	res.Object["spec"].(map[string]interface{})["values"].(map[string]interface{})["telemetry"].(map[string]interface{})["v2"].(map[string]interface{})["prometheus"].(map[string]interface{})["configOverride"].(map[string]interface{})["inboundSidecar"].(map[string]interface{})["metrics"] = inbound
	res.Object["spec"].(map[string]interface{})["values"].(map[string]interface{})["telemetry"].(map[string]interface{})["v2"].(map[string]interface{})["prometheus"].(map[string]interface{})["configOverride"].(map[string]interface{})["outboundSidecar"].(map[string]interface{})["metrics"] = outbound
	return res, true
}

// delete our config
func denormalizeOperator(res *unstructured.Unstructured) *unstructured.Unstructured {
	if v := res.Object["spec"].(map[string]interface{})["values"]; v == nil {
		return res
	}
	if v := res.Object["spec"].(map[string]interface{})["values"].(map[string]interface{})["telemetry"]; v == nil {
		return res
	}
	if v := res.Object["spec"].(map[string]interface{})["values"].(map[string]interface{})["telemetry"].(map[string]interface{})["prometheus"]; v == nil {
		return res
	}
	if v := res.Object["spec"].(map[string]interface{})["values"].(map[string]interface{})["telemetry"].(map[string]interface{})["prometheus"].(map[string]interface{})["configOverride"]; v == nil {
		return res
	}
	res.Object["spec"].(map[string]interface{})["values"].(map[string]interface{})["telemetry"].(map[string]interface{})["prometheus"].(map[string]interface{})["configOverride"] = make(map[string]interface{})
	return res
}
