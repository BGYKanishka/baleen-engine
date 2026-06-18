package handlers

import "net/http"

func Images() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Implementation for Docker SDK list images goes here
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}
}
