package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/BGYKanishka/baleen-engine/internal/cli"
	"github.com/BGYKanishka/baleen-engine/internal/config"
	"github.com/BGYKanishka/baleen-engine/internal/network"
)

func TestNetworkSettings_Unavailable(t *testing.T) {
	ctx := cli.EngineContext{}
	handler := NetworkSettings(ctx)

	req := httptest.NewRequest(http.MethodGet, "/network", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rr.Code)
	}
}

func TestNetworkSettings_GetAndPost(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	os.MkdirAll(filepath.Join(tempHome, ".baleen"), 0755)

	parentCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := network.NewPeerRegistry()
	nc := network.NewNetworkController(parentCtx, "test", 9999, "fingerprint", reg, true, true)

	ctx := cli.EngineContext{
		NetworkController: nc,
		PeerRegistry:      reg,
	}

	handler := NetworkSettings(ctx)

	// Test GET
	req := httptest.NewRequest(http.MethodGet, "/network", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var getResp map[string]bool
	json.NewDecoder(rr.Body).Decode(&getResp)
	if !getResp["mdns_discovery"] || !getResp["broadcast_presence"] {
		t.Errorf("expected both true initially, got %v", getResp)
	}

	// Test POST
	body := bytes.NewBufferString(`{"mdns_discovery": false, "broadcast_presence": false}`)
	req2 := httptest.NewRequest(http.MethodPost, "/network", body)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr2.Code)
	}

	if nc.IsDiscoveryEnabled() {
		t.Errorf("expected discovery to be false")
	}
	if nc.IsBroadcastEnabled() {
		t.Errorf("expected broadcast to be false")
	}

	// Verify persistence
	loaded := config.LoadNetworkSettings()
	if loaded.MDNSDiscovery || loaded.BroadcastPresence {
		t.Errorf("expected persisted settings to be false, got %v", loaded)
	}
}

func TestNetworkSettings_PostInvalid(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	os.MkdirAll(filepath.Join(tempHome, ".baleen"), 0755)

	parentCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := network.NewPeerRegistry()
	nc := network.NewNetworkController(parentCtx, "test", 9999, "fingerprint", reg, true, true)
	ctx := cli.EngineContext{
		NetworkController: nc,
		PeerRegistry:      reg,
	}

	handler := NetworkSettings(ctx)
	body := bytes.NewBufferString(`{invalid}`)
	req := httptest.NewRequest(http.MethodPost, "/network", body)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestNetworkSettings_PostFailsToSave(t *testing.T) {
	t.Setenv("HOME", "/nonexistent_mock_home_path")

	parentCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := network.NewPeerRegistry()
	nc := network.NewNetworkController(parentCtx, "test", 9999, "fingerprint", reg, true, true)
	ctx := cli.EngineContext{
		NetworkController: nc,
		PeerRegistry:      reg,
	}

	handler := NetworkSettings(ctx)
	body := bytes.NewBufferString(`{"mdns_discovery": false}`)
	req := httptest.NewRequest(http.MethodPost, "/network", body)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if _, ok := resp["warning"]; !ok {
		t.Errorf("expected warning when save fails, got %v", resp)
	}
}
