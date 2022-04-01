package handlers

import (
	"net/http"
	"time"

	"github.com/kyverno/kyverno/pkg/webhookconfig"
)

func Monitor(m *webhookconfig.Monitor, inner http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m.SetTime(time.Now())
		inner(w, r)
	}
}
