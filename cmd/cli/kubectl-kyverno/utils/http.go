package utils

import (
	"net/http"
	"time"
)

const RemoteHTTPTimeout = 30 * time.Second

var RemoteHTTPClient = &http.Client{
	Timeout: RemoteHTTPTimeout,
}
