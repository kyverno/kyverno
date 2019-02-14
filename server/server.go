package server

import (
	"net/http/httputil"
	"net/http"
	"crypto/tls"
	"context"
	"time"
	"log"
	"os"
)

// WebhookServer is a struct that describes 
// TLS server with mutation webhook
type WebhookServer struct {
	server http.Server
	logger *log.Logger
}

func (ws *WebhookServer) serve(w http.ResponseWriter, r *http.Request) {
	dump, _ := httputil.DumpRequest(r, true)
	ws.logger.Printf("%s", dump)
}

// RunAsync runs TLS server in separate 
// thread and returns control immediately
func (ws *WebhookServer) RunAsync() {
	go func(ws *WebhookServer) {
		err := ws.server.ListenAndServeTLS("", "")
		if err != nil {
			ws.logger.Fatal(err)
		}
	}(ws)
}

// Stop stops TLS server
func (ws *WebhookServer) Stop() {
	err := ws.server.Shutdown(context.Background())
	if err != nil {
		// Error from closing listeners, or context timeout:
		ws.logger.Printf("Server Shutdown error: %v", err)
		ws.server.Close()
	}
}

// NewWebhookServer creates new instance of WebhookServer and configures it
func NewWebhookServer(certFile string, keyFile string, logger *log.Logger) *WebhookServer {
	if logger == nil {
		logger = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
	}
	
	var config tls.Config
	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		logger.Fatal("Unable to load certificate and key: ", err)
	}
	config.Certificates = []tls.Certificate{pair}

	mux := http.NewServeMux()
	
    ws := &WebhookServer {
		server: http.Server {
			Addr:         ":443", // Listen on port for HTTPS requests
			TLSConfig:    &config,
			Handler:      mux,
			ErrorLog:     logger,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		},
		logger: logger,
	}

	mux.HandleFunc("/mutate", ws.serve)

	return ws
}
