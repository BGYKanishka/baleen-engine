package transfer

import (
	"encoding/json"
	"sync"
	"time"
)

type ProgressEvent struct {
	Direction string  `json:"direction"`
	Image     string  `json:"image"`
	Peer      string  `json:"peer"`
	Progress  float64 `json:"progress"`
	Speed     string  `json:"speed"`
	Status    string  `json:"status"`
	UpdatedAt int64   `json:"updatedAt"`
}

type Hub struct {
	mu        sync.RWMutex
	transfers map[string]*ProgressEvent
}

var GlobalHub = &Hub{
	transfers: make(map[string]*ProgressEvent),
}

func hubKey(image, peer string) string {
	return image + "|" + peer
}

func (h *Hub) Publish(event ProgressEvent) {
	event.UpdatedAt = time.Now().UnixMilli()
	h.mu.Lock()
	h.transfers[hubKey(event.Image, event.Peer)] = &event
	h.mu.Unlock()
}

// GetAll returns all current transfer states as a JSON array.
// Cleans up completed/failed/rejected transfers that are older than 10 seconds.
func (h *Hub) GetAll() []byte {
	h.mu.Lock()
	now := time.Now().UnixMilli()
	for k, v := range h.transfers {
		done := v.Status == "completed" || v.Status == "failed" || v.Status == "rejected" || v.Status == "cancelled"
		if done && now-v.UpdatedAt > 10_000 {
			delete(h.transfers, k)
		}
	}
	result := make([]ProgressEvent, 0, len(h.transfers))
	for _, v := range h.transfers {
		result = append(result, *v)
	}
	h.mu.Unlock()

	data, _ := json.Marshal(result)
	return data
}
