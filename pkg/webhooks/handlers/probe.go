package handlers

import "net/http"

func Probe(check func() error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if check != nil {
			if err := check(); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}
