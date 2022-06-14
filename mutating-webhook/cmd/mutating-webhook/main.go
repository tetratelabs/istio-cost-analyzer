package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
	"net/http"
	"os"

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
)

func main() {
	if cloud == "" {
		cloud = "gcp"
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

	if err := runWebhookServer(*tlsCert, *tlsKey, *port); err != nil {
		log.Fatal(err)
	}
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

	// Do server-side validation that we are only dealing with a pod resource. This
	// should also be part of the MutatingWebhookConfiguration in the cluster, but
	// we should verify here before continuing.
	podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	deploymentResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "deployments"}
	resourceType := admissionReviewRequest.Request.Resource
	if resourceType != podResource || resourceType != deploymentResource {
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
	if resourceType == podResource {
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
		patch = fmt.Sprintf(`[{"op":"add","path":"/metadata/labels/locality","value": "%v"}]`, podLocality)
	} else {
		deployment := v1.Deployment{}
		if _, _, err := deserializer.Decode(raw, nil, &deployment); err != nil {
			logger.Printf("decoding raw deployment: %v", err)
			http.Error(w, "failed to decode deployment", http.StatusInternalServerError)
		}
		patch = `[{"op":"add","path":"/spec/template/metadata/annotations/sidecar.istio.io/extraStatTags","value": "destination_pod"}]`
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

	return server.ListenAndServeTLS("", "")
}
