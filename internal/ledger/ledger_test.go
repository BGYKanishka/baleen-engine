package ledger

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestLedger(t *testing.T) *Ledger {
	t.Helper()
	db, err := NewLedger(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// verifies commits round-trip through the DB, and that GetHistory returns them sorted newest-first.
func TestRecordAndGetHistory(t *testing.T) {
	db := newTestLedger(t)

	commits := []Commit{
		{Hash: "aaa111", Image: "nginx:latest", Author: "alice", Direction: "Exported", Status: "Completed", Timestamp: "2024-01-01T10:00:00Z"},
		{Hash: "bbb222", Image: "redis:7", Author: "bob", Direction: "Imported", Status: "Completed", Timestamp: "2024-01-02T10:00:00Z"},
	}
	for _, c := range commits {
		if err := db.RecordCommit(c); err != nil {
			t.Fatalf("RecordCommit %s: %v", c.Hash, err)
		}
	}

	history, err := db.GetHistory()
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(history))
	}
	// Newest first.
	if history[0].Hash != "bbb222" {
		t.Errorf("expected newest commit first, got %s", history[0].Hash)
	}
}

// verifies an empty ledger returns an empty slice .
func TestGetHistory_Empty(t *testing.T) {
	db := newTestLedger(t)
	history, err := db.GetHistory()
	if err != nil {
		t.Fatalf("GetHistory on empty DB: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("expected 0 commits, got %d", len(history))
	}
}

// removes a commit by its exact hash.
func TestDeleteCommit_ByFullHash(t *testing.T) {
	db := newTestLedger(t)
	db.RecordCommit(Commit{Hash: "deadbeef", Image: "x", Timestamp: "2024-01-01T00:00:00Z"})

	if err := db.DeleteCommit("deadbeef"); err != nil {
		t.Fatalf("DeleteCommit: %v", err)
	}
	history, _ := db.GetHistory()
	if len(history) != 0 {
		t.Errorf("expected empty ledger after delete, got %d entries", len(history))
	}
}

// removes a commit using its first 8 characters.
func TestDeleteCommit_ByPrefix(t *testing.T) {
	db := newTestLedger(t)
	db.RecordCommit(Commit{Hash: "cafebabe1234", Image: "y", Timestamp: "2024-01-01T00:00:00Z"})

	if err := db.DeleteCommit("cafebabe"); err != nil {
		t.Fatalf("DeleteCommit by prefix: %v", err)
	}
	history, _ := db.GetHistory()
	if len(history) != 0 {
		t.Errorf("expected empty ledger after prefix-delete, got %d", len(history))
	}
}

// returns an error for a hash that doesn't exist.
func TestDeleteCommit_NotFound(t *testing.T) {
	db := newTestLedger(t)
	if err := db.DeleteCommit("nonexistent"); err == nil {
		t.Fatal("expected error deleting non-existent commit, got nil")
	}
}

// removes commits older than the cutoff and keeps recent ones.
func TestPruneHistoryOlderThan(t *testing.T) {
	db := newTestLedger(t)

	db.RecordCommit(Commit{Hash: "old001", Image: "old", Timestamp: "2020-01-01T00:00:00Z"})
	db.RecordCommit(Commit{Hash: "new001", Image: "new", Timestamp: time.Now().Format(time.RFC3339)})

	count, err := db.PruneHistoryOlderThan(time.Now().Add(-24 * time.Hour))
	if err != nil {
		t.Fatalf("PruneHistoryOlderThan: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 pruned commit, got %d", count)
	}

	history, _ := db.GetHistory()
	if len(history) != 1 || history[0].Hash != "new001" {
		t.Errorf("wrong remaining commits: %+v", history)
	}
}

// verifies the layer ownership cache.
func TestMarkLayersAsOwned_AndHasLayer(t *testing.T) {
	db := newTestLedger(t)

	layers := []string{"sha256:aaaa", "sha256:bbbb"}
	if err := db.MarkLayersAsOwned(layers); err != nil {
		t.Fatalf("MarkLayersAsOwned: %v", err)
	}
	for _, l := range layers {
		if !db.HasLayer(l) {
			t.Errorf("HasLayer(%s) = false, want true", l)
		}
	}
	if db.HasLayer("sha256:unknown") {
		t.Error("HasLayer(unknown) = true, want false")
	}
}

// wipes transfer history but preserves the layer cache.
func TestClearLedgerOnly(t *testing.T) {
	db := newTestLedger(t)
	db.RecordCommit(Commit{Hash: "h1", Image: "img", Timestamp: "2024-01-01T00:00:00Z"})
	db.MarkLayersAsOwned([]string{"sha256:layer1"})

	if err := db.ClearLedgerOnly(); err != nil {
		t.Fatalf("ClearLedgerOnly: %v", err)
	}

	history, _ := db.GetHistory()
	if len(history) != 0 {
		t.Errorf("expected empty history, got %d entries", len(history))
	}
	if !db.HasLayer("sha256:layer1") {
		t.Error("layer cache was wiped by ClearLedgerOnly — should be preserved")
	}
}

// wipes the layer cache but preserves transfer history.
func TestClearCacheMemory(t *testing.T) {
	db := newTestLedger(t)
	db.RecordCommit(Commit{Hash: "h2", Image: "img", Timestamp: "2024-01-01T00:00:00Z"})
	db.MarkLayersAsOwned([]string{"sha256:layer2"})

	if err := db.ClearCacheMemory(); err != nil {
		t.Fatalf("ClearCacheMemory: %v", err)
	}

	if db.HasLayer("sha256:layer2") {
		t.Error("layer still in cache after ClearCacheMemory")
	}
	history, _ := db.GetHistory()
	if len(history) != 1 {
		t.Errorf("expected 1 history entry to survive, got %d", len(history))
	}
}

// verifies the SHA-256 helper is deterministic.
func TestGenerateHash_Stable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.bin")
	if err := os.WriteFile(path, []byte("hello baleen"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	h1, err := GenerateHash(path)
	if err != nil {
		t.Fatalf("GenerateHash: %v", err)
	}
	h2, _ := GenerateHash(path)
	if h1 != h2 {
		t.Errorf("GenerateHash not stable: %s vs %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("expected 64-char hex SHA-256, got len=%d: %s", len(h1), h1)
	}
}
