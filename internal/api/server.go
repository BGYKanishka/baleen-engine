package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/cli"
)

var (
	lastActive time.Time
	activeMu   sync.Mutex
)

// JSON response model
type PeerResponse struct {
	Hostname string    `json:"hostname"`
	IP       string    `json:"ip"`
	Source   string    `json:"source"`
	Status   string    `json:"status"`
	LastSeen time.Time `json:"lastSeen"`
}

func StartDaemonServer(ctx cli.EngineContext, token string) {
	lastActive = time.Now()

	mux := http.NewServeMux()

	//Define API Endpoints
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		updateHeartbeat()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	// Peer Management Endpoint
	mux.HandleFunc("/api/peers", func(w http.ResponseWriter, r *http.Request) {
		updateHeartbeat()
		if r.Method == http.MethodGet {
			nodes := ctx.PeerRegistry.GetDetailedPeers()
			var response []PeerResponse
			for name, node := range nodes {
				response = append(response, PeerResponse{
					Hostname: name,
					IP:       node.Address,
					Source:   node.Source,
					Status:   node.Status,
					LastSeen: node.LastSeen,
				})
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		// Handle peer addition
		if r.Method == http.MethodPost {
			var payload struct {
				IP string `json:"ip"`
			}
			json.NewDecoder(r.Body).Decode(&payload)
			ctx.PeerRegistry.AddCustomPeer(fmt.Sprintf("manual-%d", time.Now().Unix()), payload.IP, "manual") // TODO: Write to ctx.EngineLedger to persist across restarts here
			w.WriteHeader(http.StatusCreated)
			return
		}
		// Handle peer deletion
		if r.Method == http.MethodDelete {
			name := strings.TrimPrefix(r.URL.Path, "/api/peers/")
			ctx.PeerRegistry.RemovePeer(name)
			w.WriteHeader(http.StatusOK)
			return
		}
	})

	// Docker SDK Endpoint
	mux.HandleFunc("/api/images", func(w http.ResponseWriter, r *http.Request) {
		updateHeartbeat()
		// Implementation for Docker SDK list images goes here
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	})

	mux.HandleFunc("/api/stream", func(w http.ResponseWriter, r *http.Request) {
		updateHeartbeat()
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		// SSE loop goes here
	})

	// Wrap with CORS and Auth Middleware
	handler := corsAndAuthMiddleware(mux, token)

	// Start the server on a random available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start daemon listener: %v\n", err)
		os.Exit(1)
	}

	actualPort := listener.Addr().(*net.TCPAddr).Port

	fmt.Printf(`{"status": "ready", "port": %d}`+"\n", actualPort)
	os.Stdout.Sync()

	// Start a goroutine to monitor
	go func() {
		for {
			time.Sleep(5 * time.Second)
			activeMu.Lock()
			idleTime := time.Since(lastActive)
			activeMu.Unlock()

			if idleTime > 2*time.Minute {
				// Extension closed or crashed. Clean up and exit.
				os.Exit(0)
			}
		}
	}()

	// Start the HTTP server
	http.Serve(listener, handler)
}

func updateHeartbeat() {
	activeMu.Lock()
	defer activeMu.Unlock()
	lastActive = time.Now()
}

func corsAndAuthMiddleware(next http.Handler, requiredToken string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
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
			if authHeader != expected {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
