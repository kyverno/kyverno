// main.go
package main

import (
	"fmt"
	"time"

	"github.com/nirmata/kube-policy/server"
)

var (
	kubeConfigFile string
)

func main() {
	server := server.NewWebhookServer()
	fmt.Println("WebHook server is running!")

	server.RunAsync()
	time.Sleep(5 * time.Second)

	server.Stop()
	fmt.Println("WebHook server is stopped.")
}
