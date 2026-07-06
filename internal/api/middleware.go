package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func corsAndAuthMiddleware(next http.Handler, requiredToken string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if origin == "http://localhost" || origin == "http://127.0.0.1" || origin == "docker-desktop://" || strings.HasPrefix(origin, "http://localhost:") || strings.HasPrefix(origin, "http://127.0.0.1:") {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Check Auth Token
		if requiredToken != "" {
			authHeader := r.Header.Get("Authorization")
			expected := "Bearer " + requiredToken
			if subtle.ConstantTimeCompare([]byte(authHeader), []byte(expected)) != 1 {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
