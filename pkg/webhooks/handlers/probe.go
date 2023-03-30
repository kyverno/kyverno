package handlers

import "net/http"

func Probe(check func() bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if check != nil {
			if !check() {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}
