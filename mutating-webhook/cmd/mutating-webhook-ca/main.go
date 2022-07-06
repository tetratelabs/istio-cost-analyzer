package main

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"log"
	"math/big"
	"os"
	"path"
	"strings"
	"time"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

func main() {
	outputDir := flag.String("output-dir", "", "The output directory to write the generated certificate files to.")
	flag.Parse()
	var (
		webhookNamespace, _ = os.LookupEnv("WEBHOOK_NAMESPACE")
		mutationCfgName, _  = os.LookupEnv("MUTATE_CONFIG")
		webhookService, _   = os.LookupEnv("WEBHOOK_SERVICE")
	)
	log.Printf("webhookNamespace: %s, mutationCfgName: %s", webhookNamespace, mutationCfgName)
	if webhookNamespace == "" {
		webhookNamespace = "istio-system"
	}
	config := ctrl.GetConfigOrDie()
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic("failed to set go -client")
	}
	org := "tetrate.io"
	dnsNames := []string{"cost-analyzer-mutating-webhook",
		"cost-analyzer-mutating-webhook." + webhookNamespace, "cost-analyzer-mutating-webhook." + webhookNamespace + ".svc"}
	commonName := "cost-analyzer-mutating-webhook." + webhookNamespace + ".svc"
	caPEM, certPEM, certKeyPEM, err := generateCert([]string{org}, dnsNames, commonName)
	if err != nil {
		log.Fatalf("unable to generate cert: %v", err)
	}
	log.Println(*outputDir)
	err = os.MkdirAll(*outputDir, 0666)
	if err != nil {
		log.Panic(err)
	}
	err = WriteFile(path.Join(*outputDir, "tls.crt"), certPEM)
	if err != nil {
		log.Fatal(err)
	}
	err = WriteFile(path.Join(*outputDir, "tls.key"), certKeyPEM)
	if err != nil {
		log.Fatal(err)
	}
	path := "/mutate"
	fail := admissionregistrationv1.Fail
	sf := admissionregistrationv1.SideEffectClassNone
	mutateconfig := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: mutationCfgName,
		},
		Webhooks: []admissionregistrationv1.MutatingWebhook{{
			Name: "mutating-webhook.istio-cost-analyzer.io",
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				CABundle: caPEM.Bytes(), // CA bundle created earlier
				Service: &admissionregistrationv1.ServiceReference{
					Name:      webhookService,
					Namespace: webhookNamespace,
					Path:      &path,
				},
			},
			Rules: []admissionregistrationv1.RuleWithOperations{{Operations: []admissionregistrationv1.OperationType{
				admissionregistrationv1.Create, admissionregistrationv1.Connect},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{"apps"},
					APIVersions: []string{"v1"},
					Resources:   []string{"deployments"},
				},
			}},
			FailurePolicy:           &fail,
			SideEffects:             &sf,
			AdmissionReviewVersions: []string{"v1"},
		}},
	}

	if _, err := kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(context.Background(), mutateconfig, metav1.CreateOptions{}); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			panic(err)
		}
	}
}

// generateCert generate a self-signed CA for given organization
// and sign certificate with the CA for given common name and dns names
// it resurns the CA, certificate and private key in PEM format
func generateCert(orgs, dnsNames []string, commonName string) (*bytes.Buffer, *bytes.Buffer, *bytes.Buffer, error) {
	// init CA config
	ca := &x509.Certificate{
		SerialNumber:          big.NewInt(2022),
		Subject:               pkix.Name{Organization: orgs},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0), // expired in 1 year
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// generate private key for CA
	caPrivateKey, err := rsa.GenerateKey(cryptorand.Reader, 4096)
	if err != nil {
		return nil, nil, nil, err
	}

	// create the CA certificate
	caBytes, err := x509.CreateCertificate(cryptorand.Reader, ca, ca, &caPrivateKey.PublicKey, caPrivateKey)
	if err != nil {
		return nil, nil, nil, err
	}

	// CA certificate with PEM encoded
	caPEM := new(bytes.Buffer)
	_ = pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	// new certificate config
	newCert := &x509.Certificate{
		DNSNames:     dnsNames,
		SerialNumber: big.NewInt(1024),
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: orgs,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(1, 0, 0), // expired in 1 year
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}

	// generate new private key
	newPrivateKey, err := rsa.GenerateKey(cryptorand.Reader, 4096)
	if err != nil {
		return nil, nil, nil, err
	}

	// sign the new certificate
	newCertBytes, err := x509.CreateCertificate(cryptorand.Reader, newCert, ca, &newPrivateKey.PublicKey, caPrivateKey)
	if err != nil {
		return nil, nil, nil, err
	}

	// new certificate with PEM encoded
	newCertPEM := new(bytes.Buffer)
	_ = pem.Encode(newCertPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: newCertBytes,
	})

	// new private key with PEM encoded
	newPrivateKeyPEM := new(bytes.Buffer)
	_ = pem.Encode(newPrivateKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(newPrivateKey),
	})

	return caPEM, newCertPEM, newPrivateKeyPEM, nil
}

// WriteFile writes data in the file at the given path
func WriteFile(filepath string, sCert *bytes.Buffer) error {
	f, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(sCert.Bytes())
	if err != nil {
		return err
	}
	return nil
}
