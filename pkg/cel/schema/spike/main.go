// Command spike serves POST /approve over a schema-driven CEL policy, mirroring
// the nono approval contract. Throwaway Phase 0 tooling, not a Kyverno binary.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/kyverno/kyverno/pkg/cel/schema"
)

const maxBody = 1 << 20

type response struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
}

// handler is fail-closed: any decode, validation or evaluation problem denies.
func handler(s *schema.Schema, policy string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /approve", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		deny := func(reason string) {
			log.Printf("denied: %s", reason)
			_ = json.NewEncoder(w).Encode(response{Decision: "denied", Reason: reason})
		}
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
		if err != nil {
			deny("reading body: " + err.Error())
			return
		}
		p, err := s.Decode(raw)
		if err != nil {
			deny(err.Error())
			return
		}
		out, err := s.Eval(policy, p)
		if err != nil {
			deny("policy evaluation: " + err.Error())
			return
		}
		if allowed, ok := out.(bool); !ok || !allowed {
			deny("denied by policy")
			return
		}
		_ = json.NewEncoder(w).Encode(response{Decision: "granted"})
	})
	return mux
}

func main() {
	var (
		addr         = flag.String("addr", "127.0.0.1:8080", "listen address")
		schemaPath   = flag.String("schema", "pkg/cel/schema/testdata/nono/schema.json", "JSON Schema file")
		computedPath = flag.String("computed", "pkg/cel/schema/testdata/nono/computed.json", "computed fields file (optional)")
		policy       = flag.String("policy", `object.request.capability_type != 'command' || object.request.command != 'kubectl' || !derived.isMutatingKubectl`, "CEL expression that must evaluate to true to grant")
	)
	flag.Parse()
	schemaJSON, err := os.ReadFile(*schemaPath)
	if err != nil {
		log.Fatal(err)
	}
	var computed []schema.Computed
	if b, err := os.ReadFile(*computedPath); err == nil {
		if err := json.Unmarshal(b, &computed); err != nil {
			log.Fatalf("computed: %v", err)
		}
	}
	s, err := schema.New(schemaJSON, computed)
	if err != nil {
		log.Fatal(err)
	}
	if err := s.Check(*policy); err != nil {
		log.Fatalf("policy: %v", err)
	}
	fmt.Printf("listening on http://%s/approve\n", *addr)
	srv := &http.Server{Addr: *addr, Handler: handler(s, *policy), ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
