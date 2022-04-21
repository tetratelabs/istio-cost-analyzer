package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
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
	codecs = serializer.NewCodecFactory(runtime.NewScheme())
	logger = log.New(os.Stdout, "", log.LstdFlags)
)

func main() {
	tlsCert := flag.String("tls-cert", "", "Certificate for TLS")
	tlsKey := flag.String("tls-key", "", "Private key file for TLS")
	port := flag.Int("port", 443, "Port to listen on for HTTPS traffic")
	flag.Parse()

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
	if admissionReviewRequest.Request.Resource != podResource {
		logger.Printf("unexpected resource of type %q, expected a pod", admissionReviewRequest.Request.Resource.Resource)
		http.Error(w, "unexpected resource", http.StatusBadRequest)
		return
	}

	// Decode the pod from the AdmissionReview.
	rawRequest := admissionReviewRequest.Request.Object.Raw
	pod := corev1.Pod{}
	if _, _, err := deserializer.Decode(rawRequest, nil, &pod); err != nil {
		logger.Printf("decoding raw pod: %v", err)
		http.Error(w, "failed to decode pod", http.StatusInternalServerError)
		return
	}

	// Create a response that will add a label to the pod if it does
	// not already have a label with the key of "hello". In this case
	// it does not matter what the value is, as long as the key exists.
	admissionResponse := &admissionv1.AdmissionResponse{
		Allowed: true,
	}

	var patch string
	patchType := admissionv1.PatchTypeJSONPatch
	patch = `[{"op":"add","path":"/spect/template/metadata/annotations/sidecar.istio.io/extraStatTags","value": "destination_pod"}]`
	admissionResponse.PatchType = &patchType
	admissionResponse.Patch = []byte(patch)

	// Construct the response, which is just another AdmissionReview.
	var admissionReviewResponse admissionv1.AdmissionReview
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
