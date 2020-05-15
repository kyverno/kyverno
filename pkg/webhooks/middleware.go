package webhooks

import (
	"net/http"
	"time"
)

func timeoutHandler(h http.Handler, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var timeoutHandler http.Handler
		msg := "ok"
		timeoutHandler = http.TimeoutHandler(h, timeout*time.Second, msg)
		timeoutHandler.ServeHTTP(w, r)
	}
}
