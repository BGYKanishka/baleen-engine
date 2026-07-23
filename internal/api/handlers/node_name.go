package handlers

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/BGYKanishka/baleen-engine/internal/cli"
	"github.com/BGYKanishka/baleen-engine/internal/config"
)

// gets the preferred outbound ip of this machine
func GetOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

func NodeName(ctx cli.EngineContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {

		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]string{
				"name": ctx.GetNodeName(),
				"ip":   GetOutboundIP(),
			})

		case http.MethodPost:
			var req struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
				return
			}
			name := strings.TrimSpace(req.Name)
			if name == "" {
				http.Error(w, `{"error":"name cannot be empty"}`, http.StatusBadRequest)
				return
			}
			if err := config.SaveNodeName(name); err != nil {
				http.Error(w, `{"error":"failed to save name"}`, http.StatusInternalServerError)
				return
			}
			if nc := ctx.NetworkController; nc != nil {
				nc.UpdateNodeName(name)
			}
			json.NewEncoder(w).Encode(map[string]string{
				"status": "saved",
				"name":   name,
			})

		}
	}
}
