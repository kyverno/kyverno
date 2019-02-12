// main.go
package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/nirmata/kube-policy/server"
)

func main() {
	var cert = flag.String("cert", "", "TLS certificate")
	var key = flag.String("key", "", "TLS key in PEM format")
	flag.Parse()

	if *cert == "" || *key == "" {
		log.Fatal("TLS certificate or/and key is not set")
	}

	logger := log.New(os.Stdout, "http: ", log.LstdFlags|log.Lshortfile)
	logger.Printf("! Server is starting...")
	server := server.NewWebhookServer(*cert, *key, logger)
	logger.Printf("! WebHook server is running!")

	server.RunAsync()
	time.Sleep(500500 * time.Second)

	server.Stop()
	logger.Printf("! WebHook server is stopped.")
}
