package webhooks

import (
	"net/http"
	"time"
)

func timeoutHandler(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var timeoutHandler http.Handler
		msg := "Your request has timed out."
		timeoutHandler = http.TimeoutHandler(h, 5*time.Second, msg)
		timeoutHandler.ServeHTTP(w, r)
	}
}
