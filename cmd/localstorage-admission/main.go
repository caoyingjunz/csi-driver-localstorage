package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"

	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog/v2"
)

var (
	healthHost = flag.String("healthHost", "127.0.0.1", "The host of Health.")
	healthPort = flag.String("healthPort", "10259", "The port of Health to listen on.")
	certFile   = flag.String("certFile", "/run/secrets/tls/tls.crt", "File containing the x509 Certificate for HTTPS.")
	keyFile    = flag.String("keyFile", "/run/secrets/tls/tls.key", "File containing the x509 private key to --tlsCertFile.")

	universalDeserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", serve)
	mux.HandleFunc("/validate", serve)
	server := &http.Server{
		Addr:    ":8443",
		Handler: mux,
	}

	go startHealthServer(*healthHost, *healthPort)
	klog.Fatal(server.ListenAndServeTLS(*certFile, *keyFile))
}

func startHealthServer(healthHost, healthPort string) {
	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})

	klog.Infof("Starting Health Server...")
	klog.Fatal(http.ListenAndServe(healthHost+":"+healthPort, nil))

}

// mutation services
func mutate(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Allowed: true,
	}
}

// validate  services
func validate(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Allowed: true,
	}
}

func serve(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		klog.Error("empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		klog.Errorf("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *v1beta1.AdmissionResponse
	ar := v1beta1.AdmissionReview{}
	if _, _, err := universalDeserializer.Decode(body, nil, &ar); err != nil {
		klog.Errorf("Can't decode body: %v", err)
		admissionResponse = &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		if r.URL.Path == "/mutate" {
			admissionResponse = mutate(&ar)
		} else if r.URL.Path == "/validate" {
			admissionResponse = validate(&ar)
		}
	}

	admissionReview := v1beta1.AdmissionReview{}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		klog.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	klog.Infof("Ready to write reponse ...")
	if _, err := w.Write(resp); err != nil {
		klog.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}

}
