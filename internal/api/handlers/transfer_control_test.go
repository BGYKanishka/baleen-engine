package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BGYKanishka/baleen-engine/internal/transfer"
)

func TestTransferControl_InvalidPayload(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/transfer/pause", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	Pause()(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", w.Result().StatusCode)
	}
}

func TestTransferControl_NotFound(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/transfer/pause", bytes.NewReader([]byte(`{"image":"unknown:latest","peer":"10.0.0.1"}`)))
	w := httptest.NewRecorder()

	Pause()(w, req)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", w.Result().StatusCode)
	}
}

func TestTransferControl_CancelPendingApproval(t *testing.T) {
	// Seed global manager with a pending approval
	respChan := make(chan bool, 1)
	transfer.GlobalManager.RegisterApproval("ubuntu:latest", "10.0.0.1", respChan)
	defer transfer.GlobalManager.UnregisterApproval("ubuntu:latest")

	req := httptest.NewRequest("POST", "/api/transfer/cancel", bytes.NewReader([]byte(`{"image":"ubuntu:latest","peer":"10.0.0.1"}`)))
	w := httptest.NewRecorder()

	Cancel()(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Result().StatusCode)
	}

	// Verify approval was cancelled
	decision := <-respChan
	if decision {
		t.Error("expected decision to be false when cancelled")
	}
}

func TestTransferControl_CancelPendingConn(t *testing.T) {
	// Seed global manager with a pending connection
	cancelled := false
	pt := &transfer.PendingTransfer{
		CancelConn:  func() { cancelled = true },
		SendControl: func(action, initiator string) {},
	}

	transfer.GlobalManager.RegisterPendingConn("ubuntu:latest", "10.0.0.2", pt)
	defer transfer.GlobalManager.UnregisterPendingConn("ubuntu:latest", "10.0.0.2")

	req := httptest.NewRequest("POST", "/api/transfer/cancel", bytes.NewReader([]byte(`{"image":"ubuntu:latest","peer":"10.0.0.2"}`)))
	w := httptest.NewRecorder()

	Cancel()(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Result().StatusCode)
	}

	if !cancelled {
		t.Error("expected CancelConn to be called")
	}
}
