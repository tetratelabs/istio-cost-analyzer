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

func (k *KubeClient) GetLocalityCalls(podCalls []*PodCall) ([]*Call, error) {
	calls := make([]*Call, 0)
	for i := 0; i < len(podCalls); i++ {
		fromNode, err := k.getPodNode(podCalls[i].FromPod)
		if err != nil {
			return nil, err
		}
		toNode, err := k.getPodNode(podCalls[i].ToPod)
		if err != nil {
			return nil, err
		}
		fromLocality, err := k.getNodeLocality(fromNode)
		if err != nil {
			return nil, err
		}
		toLocality, err := k.getNodeLocality(toNode)
		if err != nil {
			return nil, err
		}
		calls = append(calls, &Call{
			From:     fromLocality,
			To:       toLocality,
			CallSize: podCalls[i].CallSize,
		})
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

func (k *KubeClient) getNodeLocality(name string) (*Locality, error) {
	node, err := k.clientSet.CoreV1().Nodes().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("error in getting node %v: %v\n", name, err)
		return nil, err
	}
	return &Locality{
		Region:  node.Labels["topology.kubernetes.io/region"],
		Zone:    node.Labels["topology.kubernetes.io/zone"],
		Subzone: node.Labels["topology.istio.io/subzone "],
	}, nil
}
