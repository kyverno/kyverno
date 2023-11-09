package handlers

import (
	"context"
	"net/http"
)

func Probe(check func(context.Context) bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if check != nil {
			if !check(r.Context()) {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}
