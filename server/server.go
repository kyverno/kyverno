package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

const ( // TODO: read these files from ~/.kube/config
	clientCertFile = "/home/quest/.minikube/client.crt"
	clientKeyFile  = "/home/quest/.minikube/client.key"
)

type WebhookServer struct {
	server http.Server
}

func (ws *WebhookServer) serve(w http.ResponseWriter, r *http.Request) {
	fmt.Println("/mutate is called!")
}

func (ws *WebhookServer) RunAsync() {
	go func(server http.Server) {
		err := server.ListenAndServeTLS(clientCertFile, clientKeyFile)
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

func NewWebhookServer() WebhookServer {
	var ws WebhookServer
	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", ws.serve)
	ws.server = http.Server{
		Addr:         ":443",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second}
	return ws
}
