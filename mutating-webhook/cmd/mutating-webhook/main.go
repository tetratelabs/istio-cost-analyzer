package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	v1 "k8s.io/api/apps/v1"
	runtime2 "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"log"
	"net/http"
	"os"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	codecs    = serializer.NewCodecFactory(runtime.NewScheme())
	logger    = log.New(os.Stdout, "", log.LstdFlags)
	clientset *kubernetes.Clientset
	cloud     = os.Getenv("CLOUD")
	//namespace  = os.Getenv("NAMESPACE")
	namespaces = strings.Split(os.Getenv("NAMESPACE"), ",")
)

func main() {
	// assume defaults
	if cloud == "" {
		cloud = "gcp"
	}
	if len(namespaces) == 0 {
		namespaces = []string{"default"}
	}
	tlsCert := flag.String("tls-cert", "", "Certificate for TLS")
	tlsKey := flag.String("tls-key", "", "Private key file for TLS")
	port := flag.Int("port", 443, "Port to listen on for HTTPS traffic")
	flag.Parse()
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	stopCh := make(chan struct{})
	//concurrently watch for pod creation and label the pod with the node locality
	go watchAndLabelPods(stopCh)
	//annotate existing deployments with stats tags
	if err = annotateExistingDeployments(); err != nil {
		log.Fatal(err)
	}

	if err = runWebhookServer(*tlsCert, *tlsKey, *port); err != nil {
		log.Fatal(err)
	}
}

// annotateExistingDeployments annotates existing deployments with stats tags.
func annotateExistingDeployments() error {
	log.Println("fetching deployments...")
	for _, ns := range namespaces {
		depl, err := clientset.AppsV1().Deployments(ns).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return err
		}
		log.Printf("annotating %v deployments in %v\n", len(depl.Items), ns)
		for _, d := range depl.Items {
			if d.Spec.Template.Annotations == nil {
				d.Spec.Template.Annotations = make(map[string]string)
			}
			if v, ok := d.Spec.Template.Annotations["sidecar.istio.io/extraStatTags"]; ok && v == "destination_locality" {
				log.Printf("skipping deployment %v/%v\n", d.Name, d.Namespace)
				continue
			}
			log.Printf("annotating deployment %v/%v\n", d.Name, d.Namespace)
			d.Spec.Template.Annotations["sidecar.istio.io/extraStatTags"] = "destination_locality"
			_, err = clientset.AppsV1().Deployments(ns).Update(context.TODO(), &d, metav1.UpdateOptions{})
			if err != nil {
				logger.Printf("error in updating deployment, skipping...: %v\n", err)
			}
		}
	}
	return nil
}

// watchAndLabelPod watches for pod creation and labels the pod with the node locality.
func watchAndLabelPods(stopCh <-chan struct{}) {
	log.Printf("labeling pods in namespaces %v...", namespaces)
	informerIndex := map[string]cache.SharedInformer{}
	for _, ns := range namespaces {
		factory := informers.NewSharedInformerFactoryWithOptions(clientset, 0, informers.WithNamespace(ns))
		informerIndex[ns] = factory.Core().V1().Pods().Informer()
	}
	for ns, informer := range informerIndex {
		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				pod := obj.(*corev1.Pod)
				log.Printf("updating pod %v\n", pod.Name)
				if pod.Namespace != ns {
					return
				}
				// Get the node locality
				locality, err := getNodeLocality(pod.Spec.NodeName, cloud)
				if err != nil {
					log.Printf("error in getting node locality: %v\n", err)
					return
				}
				// Label the pod with the node locality
				pod.ObjectMeta.Labels["locality"] = locality
				// annotate the pod with destination_locality tag
				if pod.Annotations == nil {
					pod.Annotations = make(map[string]string)
				}
				pod.Annotations["sidecar.istio.io/extraStatTags"] = "destination_locality"
				// Update the pod
				_, err = clientset.CoreV1().Pods(pod.Namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})
				if err != nil {
					log.Printf("error in updating pod: %v\n", err)
					return
				}
				log.Printf("Pod %v updated\n", pod.Name)
			},
		})
		informer.Run(stopCh)
	}
	defer runtime2.HandleCrash()
}

var deserializer = codecs.UniversalDeserializer()

func mutatePod(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "invalid content type, want `application/json`", http.StatusBadRequest)
		return
	}

	body, err := readBody(r)
	if err != nil {
		logger.Printf("reading body: %v", err)
		http.Error(w, "failed to read request body", http.StatusInternalServerError)
		return
	}

	// Decode the request body into
	admissionReviewRequest := &admissionv1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, admissionReviewRequest); err != nil {
		logger.Printf("decoding body: %v", err)
		http.Error(w, "failed to read request body", http.StatusInternalServerError)
		return
	}
	logger.Printf("got for %v", admissionReviewRequest.Request.Resource.Resource)

	// Do server-side validation that we are only dealing with a pod resource. This
	// should also be part of the MutatingWebhookConfiguration in the cluster, but
	// we should verify here before continuing.
	//podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	//deploymentResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "deployments"}
	resourceType := admissionReviewRequest.Request.Resource
	if resourceType.Resource != "deployments" && resourceType.Resource != "pods" {
		logger.Printf("unexpected resource of type %q, expected a pod/deployment", admissionReviewRequest.Request.Resource.Resource)
		http.Error(w, "unexpected resource", http.StatusBadRequest)
		return
	}

	// Decode AdmissionReview to pod or deployment.
	raw := admissionReviewRequest.Request.Object.Raw
	var admissionReviewResponse admissionv1.AdmissionReview
	admissionResponse := &admissionv1.AdmissionResponse{
		Allowed: true,
	}
	var patch string
	// todo this should probably be deleted at some point
	if resourceType.Resource == "pods" {
		pod := corev1.Pod{}
		if _, _, err := deserializer.Decode(raw, nil, &pod); err != nil {
			logger.Printf("decoding raw pod: %v", err)
			http.Error(w, "failed to decode pod", http.StatusInternalServerError)
			return
		}
		// todo custom configure cloud
		podLocality, err := getNodeLocality(pod.Spec.NodeName, "gcp")
		if err != nil {
			logger.Printf("unable to get locality from node info for pod %v, skipping patching locality\n", pod.Name)
		}
		log.Printf("editing pod %v for locality %v", pod.Name, podLocality)
		patch = fmt.Sprintf(`[
{"op":"add",
"path":"/metadata/labels/locality","value": "%v"}
]`, podLocality)
	} else {
		// handle deployments
		deployment := v1.Deployment{}
		if _, _, err := deserializer.Decode(raw, nil, &deployment); err != nil {
			logger.Printf("decoding raw deployment: %v", err)
			http.Error(w, "failed to decode deployment", http.StatusInternalServerError)
		}
		log.Printf("editing deployment %v, adding destination_pod...", deployment.Name)
		// we need to add the metadata/annotations key if it doesn't exist.
		annotationPatch := ""
		if len(deployment.Spec.Template.Annotations) == 0 {
			annotationPatch = `{"op":"add","path":"/spec/template/metadata/annotations","value":{}},`
		}
		log.Printf("annotationPatch: %v", annotationPatch)
		patch = fmt.Sprintf(`[%v
{"op":"add",
"path":"/spec/template/metadata/annotations/sidecar.istio.io~1extraStatTags","value": "destination_locality"}]`, annotationPatch)
	}

	// Construct response
	patchType := admissionv1.PatchTypeJSONPatch
	admissionResponse.PatchType = &patchType
	admissionResponse.Patch = []byte(patch)

	admissionReviewResponse.Response = admissionResponse
	admissionReviewResponse.SetGroupVersionKind(admissionReviewRequest.GroupVersionKind())
	admissionReviewResponse.Response.UID = admissionReviewRequest.Request.UID
	res, err := json.Marshal(admissionReviewResponse)
	if err != nil {
		logger.Printf("marshaling response: %v", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(res)
}

func readBody(r *http.Request) ([]byte, error) {
	var body []byte
	if r.Body != nil {
		requestData, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		body = requestData
	}
	return body, nil
}

// getNodeLocality gets the locality given by topology.kubernetes.io.
func getNodeLocality(name, cloud string) (string, error) {
	// if we are on AWS, we want to just get region, because availability zones
	// are not supported yet.
	if cloud == "aws" {
		return getNodeLabel(name, "topology.kubernetes.io/region")
	}
	return getNodeLabel(name, "topology.kubernetes.io/zone")
}

// getNodeLabel returns the value of the label on the node with the given name.
func getNodeLabel(name, label string) (string, error) {
	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("error in getting node %v: %v\n", name, err)
		return "", err
	}
	return node.Labels[label], nil
}

func runWebhookServer(certFile, keyFile string, port int) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}

	http.HandleFunc("/mutate", mutatePod)

	server := http.Server{
		Addr: fmt.Sprintf(":%d", port),
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
		ErrorLog: logger,
	}

	return server.ListenAndServeTLS(certFile, keyFile)
}
