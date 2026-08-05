package handlers

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/cli"
)

type PeerResponse struct {
	Hostname string    `json:"hostname"`
	IP       string    `json:"ip"`
	Source   string    `json:"source"`
	Status   string    `json:"status"`
	Arch     string    `json:"arch"`
	LastSeen time.Time `json:"lastSeen"`
}

func Peers(ctx cli.EngineContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			nodes := ctx.PeerRegistry.GetDetailedPeers()
			response := []PeerResponse{}
			for name, node := range nodes {
				response = append(response, PeerResponse{
					Hostname: name,
					IP:       node.Address,
					Source:   node.Source,
					Status:   node.Status,
					Arch:     node.Arch,
					LastSeen: node.LastSeen,
				})
			}

			sort.Slice(response, func(i, j int) bool {
				if response[i].Status != response[j].Status {
					return response[i].Status == "reachable"
				}
				return response[i].Hostname < response[j].Hostname
			})

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)

		case http.MethodPost:
			var payload struct {
				IP string `json:"ip"`
			}
			json.NewDecoder(r.Body).Decode(&payload)
			if _, _, err := net.SplitHostPort(payload.IP); err != nil {
				http.Error(w, "invalid IP/port format", http.StatusBadRequest)
				return
			}
			ctx.PeerRegistry.AddCustomPeer(fmt.Sprintf("manual-%d", time.Now().Unix()), payload.IP, "manual")
			w.WriteHeader(http.StatusCreated)

		case http.MethodDelete:
			// Correctly parse peer name from /api/peers/{name}
			name := strings.TrimPrefix(r.URL.Path, "/api/peers/")
			if name == "" {
				http.Error(w, "peer name required", http.StatusBadRequest)
				return
			}
			ctx.PeerRegistry.RemovePeer(name)
			w.WriteHeader(http.StatusOK)

		}
	}
}
