package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BGYKanishka/baleen-engine/internal/cli"
	"github.com/BGYKanishka/baleen-engine/internal/network"
)

func TestPeersHandler_Get(t *testing.T) {
	pr := network.NewPeerRegistry()
	pr.AddPeer("node-a", "10.0.0.1:8080", "fingerprint1")

	ctx := cli.EngineContext{
		PeerRegistry: pr,
	}

	req := httptest.NewRequest("GET", "/api/peers", nil)
	w := httptest.NewRecorder()

	Peers(ctx)(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Result().StatusCode)
	}

	var response []PeerResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(response))
	}
	if response[0].Hostname != "node-a" {
		t.Errorf("expected node-a, got %s", response[0].Hostname)
	}
}

func TestPeersHandler_Post(t *testing.T) {
	pr := network.NewPeerRegistry()
	ctx := cli.EngineContext{
		PeerRegistry: pr,
	}

	req := httptest.NewRequest("POST", "/api/peers", bytes.NewReader([]byte(`{"ip":"10.0.0.5:9090"}`)))
	w := httptest.NewRecorder()

	Peers(ctx)(w, req)

	if w.Result().StatusCode != http.StatusCreated {
		t.Errorf("expected 201 Created, got %d", w.Result().StatusCode)
	}

	peers := pr.GetAllPeers()
	if len(peers) != 1 {
		t.Errorf("expected 1 peer added, got %d", len(peers))
	}
}

func TestPeersHandler_Delete(t *testing.T) {
	pr := network.NewPeerRegistry()
	pr.AddPeer("node-delete", "10.0.0.1:8080", "fp")

	ctx := cli.EngineContext{
		PeerRegistry: pr,
	}

	// Successful delete
	req := httptest.NewRequest("DELETE", "/api/peers/node-delete", nil)
	w := httptest.NewRecorder()

	Peers(ctx)(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Result().StatusCode)
	}

	if len(pr.GetAllPeers()) != 0 {
		t.Error("expected peer to be removed")
	}

	// Bad request (missing name)
	req2 := httptest.NewRequest("DELETE", "/api/peers/", nil)
	w2 := httptest.NewRecorder()

	Peers(ctx)(w2, req2)

	if w2.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", w2.Result().StatusCode)
	}
}
