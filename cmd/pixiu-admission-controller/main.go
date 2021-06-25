package main

import (
	"log"
	"net/http"
	"path/filepath"
)

const (
	tlsDir      = `/run/secrets/tls`
	tlsCertFile = `tls.crt`
	tlsKeyFile  = `tls.key`
)

func main() {
	certPath := filepath.Join(tlsDir, tlsCertFile)
	keyPath := filepath.Join(tlsDir, tlsKeyFile)

	// define http server and server handler
	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", serve)
	mux.HandleFunc("/validate", serve)

    server := &http.Server{
    	Addr:  ":8443",
    	Handler: mux,
    }
	log.Fatal(server.ListenAndServeTLS(certPath, keyPath))
}

