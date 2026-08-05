package main

import (
	"encoding/json"
	"fmt"

	"github.com/BGYKanishka/baleen-engine/internal/service"
)

// runStatus prints the current status of the daemon, if any, and exits.
// It is invoked by the `baleen status` command.
func runStatus() {
	if existing, err := service.ReadState(); err == nil && service.IsAlive(existing) {
		if service.KillIfOutdated(existing) {
			fmt.Println(`{"status":"stopped"}`)
			return
		}

		out := map[string]any{
			"status":    "running",
			"port":      existing.Port,
			"token":     existing.Token,
			"node_name": existing.NodeName,
		}
		data, _ := json.Marshal(out)
		fmt.Println(string(data))
	} else {
		fmt.Println(`{"status":"stopped"}`)
	}
}
