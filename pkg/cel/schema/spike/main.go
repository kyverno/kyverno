// Command spike serves schema-driven CEL policies over HTTP, loading
// PayloadSchema, ValidatingPolicy (mode: Schema) and PayloadBinding resources
// from a directory. Throwaway Phase 0 tooling, not a Kyverno binary.
package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/kyverno/kyverno/pkg/cel/schema"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	dir := flag.String("resources", "pkg/cel/schema/examples", "directory of YAML/JSON resources")
	certFile := flag.String("tls-cert", "", "server certificate (enables TLS)")
	keyFile := flag.String("tls-key", "", "server private key")
	clientCA := flag.String("client-ca", "", "CA bundle for client certificates (enables mTLS authorization)")
	flag.Parse()
	var opts schema.Options
	srv := &http.Server{Addr: *addr, ReadHeaderTimeout: 5 * time.Second}
	if *clientCA != "" {
		pem, err := os.ReadFile(*clientCA)
		if err != nil {
			log.Fatal(err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			log.Fatal("no certificates found in -client-ca")
		}
		// Absent certs are answered 401 by the app; invalid ones fail the handshake.
		srv.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, ClientAuth: tls.VerifyClientCertIfGiven, ClientCAs: pool}
		opts.Principal = schema.MTLSPrincipal
	} else {
		fmt.Println("WARNING: no -client-ca; running unauthenticated, binding authorization is ignored")
	}
	reg, err := schema.LoadDir(*dir, opts)
	if err != nil {
		log.Fatalf("loading %s: %v", *dir, err)
	}
	for _, line := range reg.Summary() {
		fmt.Println("route:", line)
	}
	srv.Handler = reg
	if *certFile != "" {
		fmt.Printf("listening on https://%s\n", *addr)
		log.Fatal(srv.ListenAndServeTLS(*certFile, *keyFile))
	}
	fmt.Printf("listening on http://%s\n", *addr)
	log.Fatal(srv.ListenAndServe())
}
