package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/BGYKanishka/baleen-engine/internal/transfer"
)

type controlPayload struct {
	ImageName string `json:"image"`
	PeerIP    string `json:"peer"`
}

func handleControl(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload controlPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}

		pw, ok := transfer.GlobalManager.Get(payload.ImageName, payload.PeerIP)
		if !ok {
			if action == "cancel" {
				// Sender cancelled before streaming. Close connection and dismiss receiver dialog.
				if pt, ok := transfer.GlobalManager.GetPendingConn(payload.ImageName, payload.PeerIP); ok {
					pt.CancelConn()
					go pt.SendControl("cancel", "sender")
					transfer.PublishStatus(payload.ImageName, payload.PeerIP, "push", "cancelled")
					w.WriteHeader(http.StatusOK)
					return
				}

				// Receiver cancelled before streaming. Reject the pending transfer.
				if peer, ok := transfer.GlobalManager.CancelApproval(payload.ImageName); ok {
					transfer.PublishStatus(payload.ImageName, peer, "pull", "cancelled")
					w.WriteHeader(http.StatusOK)
					return
				}
			}
			http.Error(w, "Transfer not found", http.StatusNotFound)
			return
		}

		// Determine whether the sender or receiver initiated the control action.
		initiator := "sender"
		if pw.Direction() == "pull" {
			initiator = "receiver"
		}

		switch action {
		case "pause":
			pw.Pause(initiator)

		case "resume":
			if err := pw.Resume(initiator); err != nil {
				// Transfer was paused by the other side. Return 409 so UI can show a toast.
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}

		case "cancel":
			pw.Cancel()
		}

		w.WriteHeader(http.StatusOK)
	}
}

func Pause() http.HandlerFunc  { return handleControl("pause") }
func Resume() http.HandlerFunc { return handleControl("resume") }
func Cancel() http.HandlerFunc { return handleControl("cancel") }
