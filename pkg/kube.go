package pkg

import (
	"context"
	"flag"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
)

type KubeClient struct {
	clientSet *kubernetes.Clientset
}

func NewDapaniKubeClient() *KubeClient {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return &KubeClient{
		clientSet: clientset,
	}
}

func (k *KubeClient) GetLocalityCalls(podCalls []*PodCall, cloud string) ([]*Call, error) {
	calls := make([]*Call, 0)
	// key is just source/destination service/locality. No call size info
	// value is full Call
	serviceCallMap := make(map[Call]*Call)
	for i := 0; i < len(podCalls); i++ {
		fromNode, err := k.getPodNode(podCalls[i].FromPod)
		if err != nil {
			return nil, err
		}
		toNode, err := k.getPodNode(podCalls[i].ToPod)
		if err != nil {
			return nil, err
		}
		fromLocality, err := k.getNodeLocality(fromNode, cloud)
		if err != nil {
			return nil, err
		}
		toLocality, err := k.getNodeLocality(toNode, cloud)
		if err != nil {
			return nil, err
		}
		serviceLocalityKey := Call{
			FromWorkload: podCalls[i].FromWorkload,
			From:         fromLocality,
			ToWorkload:   podCalls[i].ToWorkload,
			To:           toLocality,
		}
		if _, ok := serviceCallMap[serviceLocalityKey]; !ok {
			serviceCallMap[serviceLocalityKey] = &serviceLocalityKey
			serviceLocalityKey.CallSize = podCalls[i].CallSize
		} else {
			serviceCallMap[serviceLocalityKey].CallSize += podCalls[i].CallSize
		}
	}
	for _, v := range serviceCallMap {
		calls = append(calls, v)
	}
	return calls, nil
}

func (k *KubeClient) getPodNode(name string) (string, error) {
	pod, err := k.clientSet.CoreV1().Pods("default").Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("error in getting pod %v: %v\n", name, err)
		return "", err
	}
	return pod.Spec.NodeName, nil
}

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
