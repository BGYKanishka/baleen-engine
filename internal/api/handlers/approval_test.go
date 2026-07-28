package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BGYKanishka/baleen-engine/internal/cli"
	"github.com/BGYKanishka/baleen-engine/internal/transfer"
)

func TestApprovalHandlers(t *testing.T) {
	// Setup mock context
	store := &cli.PendingApprovalStore{}
	ctx := cli.EngineContext{
		PendingApproval: store,
	}

	// Test Pending when empty
	req := httptest.NewRequest("GET", "/api/approval/pending", nil)
	w := httptest.NewRecorder()
	Pending(ctx)(w, req)

	if w.Result().StatusCode != http.StatusNoContent {
		t.Errorf("expected 204 No Content, got %d", w.Result().StatusCode)
	}

	// Test Pending with data
	ch := make(chan bool, 1)
	approvalReq := transfer.ApprovalRequest{
		Req: transfer.TransferRequest{
			ImageName: "ubuntu:latest",
			Author:    "tester",
		},
		Response: ch,
	}
	store.Store(approvalReq)

	w = httptest.NewRecorder()
	Pending(ctx)(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Result().StatusCode)
	}

	var response transfer.TransferRequest
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.ImageName != "ubuntu:latest" {
		t.Errorf("expected ubuntu:latest, got %s", response.ImageName)
	}

	// Test Approve
	reqApprove := httptest.NewRequest("POST", "/api/approval/approve", nil)
	w = httptest.NewRecorder()
	Approve(ctx)(w, reqApprove)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK on approve, got %d", w.Result().StatusCode)
	}

	decision := <-ch
	if !decision {
		t.Error("expected decision to be true for Approve")
	}

	// Store should be cleared
	_, ok := store.Load()
	if ok {
		t.Error("store should be empty after approval")
	}

	// Test Reject
	store.Store(approvalReq)
	reqReject := httptest.NewRequest("POST", "/api/approval/reject", nil)
	w = httptest.NewRecorder()
	Reject(ctx)(w, reqReject)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK on reject, got %d", w.Result().StatusCode)
	}

	decision = <-ch
	if decision {
		t.Error("expected decision to be false for Reject")
	}

	// Test Reject when empty
	w = httptest.NewRecorder()
	Reject(ctx)(w, reqReject)
	if w.Result().StatusCode != http.StatusConflict {
		t.Errorf("expected 409 Conflict when empty, got %d", w.Result().StatusCode)
	}
}
