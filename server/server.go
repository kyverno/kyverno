package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"time"
)

type WebhookServer struct {
	server http.Server
}

func (ws *WebhookServer) serve(w http.ResponseWriter, r *http.Request) {
	fmt.Println("/mutate is called!")
	httputil.DumpRequest(r, true)
}

func (ws *WebhookServer) RunAsync() {
	go func(server http.Server) {
		err := server.ListenAndServeTLS("", "")
		if err != nil {
			log.Fatal(err)
		}
	}(ws.server)
}

func (ws *WebhookServer) Stop() {
	err := ws.server.Shutdown(context.Background())
	if err != nil {
		// Error from closing listeners, or context timeout:
		log.Printf("Server Shutdown error: %v", err)
		ws.server.Close()
	}
}

func NewWebhookServer(certFile string, keyFile string, logger *log.Logger) WebhookServer {
	var ws WebhookServer
	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", ws.serve)

	var config tls.Config
	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatal("Unable to load certificate and key: ", err)
	}
	config.Certificates = []tls.Certificate{pair}

	ws.server = http.Server{
		Addr:         ":443", // Listen on port for HTTPS requests
		TLSConfig:    &config,
		Handler:      mux,
		ErrorLog:     logger,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second}
	return ws
}
