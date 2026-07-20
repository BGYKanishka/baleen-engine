package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/BGYKanishka/baleen-engine/internal/cli"
	"github.com/BGYKanishka/baleen-engine/internal/config"
)

func TestNodeName_Get(t *testing.T) {
	ctx := cli.EngineContext{
		NodeName: "TestNode-42",
	}

	req := httptest.NewRequest(http.MethodGet, "/name", nil)
	rr := httptest.NewRecorder()

	handler := NodeName(ctx)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rr.Code)
	}

	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["name"] != "TestNode-42" {
		t.Errorf("expected name TestNode-42, got %v", resp["name"])
	}
}

func TestNodeName_Post(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	os.MkdirAll(filepath.Join(tempHome, ".baleen"), 0755)

	ctx := cli.EngineContext{}
	handler := NodeName(ctx)

	t.Run("ValidName", func(t *testing.T) {
		body := bytes.NewBufferString(`{"name": "NewNode-99"}`)
		req := httptest.NewRequest(http.MethodPost, "/name", body)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d. body: %s", rr.Code, rr.Body.String())
		}

		loaded := config.LoadNodeName()
		if loaded != "NewNode-99" {
			t.Errorf("expected saved name to be NewNode-99, got %q", loaded)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		body := bytes.NewBufferString(`{"name": }`)
		req := httptest.NewRequest(http.MethodPost, "/name", body)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", rr.Code)
		}
	})

	t.Run("EmptyName", func(t *testing.T) {
		body := bytes.NewBufferString(`{"name": "   "}`)
		req := httptest.NewRequest(http.MethodPost, "/name", body)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", rr.Code)
		}
	})

	t.Run("SaveFails", func(t *testing.T) {
		t.Setenv("HOME", "/nonexistent_mock_home_path") // will cause SaveNodeName to fail
		body := bytes.NewBufferString(`{"name": "FailingNode"}`)
		req := httptest.NewRequest(http.MethodPost, "/name", body)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 Internal Server Error, got %d", rr.Code)
		}
	})
}
