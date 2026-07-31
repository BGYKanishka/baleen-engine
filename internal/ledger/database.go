package ledger

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"go.etcd.io/bbolt"
)

type Commit struct {
	Hash      string `json:"hash"`
	Image     string `json:"image"`
	Author    string `json:"author"`
	Peer      string `json:"peer"`
	Timestamp string `json:"timestamp"`
	Direction string `json:"direction"`
	Status    string `json:"status"`
}

// Ledger holds the persistent database connection
type Ledger struct {
	db *bbolt.DB
}

// NewLedger initializes the database once and ensures buckets exist
func NewLedger(dbPath string) (*Ledger, error) {
	// Open the DB with a timeout so it doesn't hang forever if locked by another process
	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}

	// Pre-create buckets on startup to save time during read/writes
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("Ledger"))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte("KnownLayers"))
		return err
	})
	if err != nil {
		return nil, err
	}

	return &Ledger{db: db}, nil
}

// Close gracefully shuts down the database
func (l *Ledger) Close() error {
	return l.db.Close()
}

func GenerateHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// saves a new commit to the ledger
func (l *Ledger) RecordCommit(commit Commit) error {
	return l.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("Ledger"))
		commitJSON, err := json.Marshal(commit)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(commit.Hash), commitJSON)
	})
}

func (l *Ledger) GetHistory() ([]Commit, error) {
	var history []Commit

	err := l.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("Ledger"))
		if bucket == nil {
			return nil
		}

		return bucket.ForEach(func(k, v []byte) error {
			var commit Commit
			if err := json.Unmarshal(v, &commit); err != nil {
				return err
			}
			history = append(history, commit)
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	// Sort by timestamp descending (newest first)
	sort.Slice(history, func(i, j int) bool {
		return history[i].Timestamp > history[j].Timestamp
	})

	return history, nil
}

func (l *Ledger) MarkLayersAsOwned(layers []string) error {
	return l.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("KnownLayers"))
		for _, layer := range layers {
			if err := bucket.Put([]byte(layer), []byte("1")); err != nil {
				return err
			}
		}
		return nil
	})
}

func (l *Ledger) HasLayer(layer string) bool {
	hasLayer := false
	l.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("KnownLayers"))
		if bucket != nil {
			if bucket.Get([]byte(layer)) != nil {
				hasLayer = true
			}
		}
		return nil
	})
	return hasLayer
}

// Removes a single commit
func (l *Ledger) DeleteCommit(hashPrefix string) error {
	return l.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("Ledger"))
		if bucket == nil {
			return nil
		}
		if bucket.Get([]byte(hashPrefix)) != nil {
			return bucket.Delete([]byte(hashPrefix))
		}
		prefixBytes := []byte(hashPrefix)
		c := bucket.Cursor()

		for k, _ := c.Seek(prefixBytes); k != nil && bytes.HasPrefix(k, prefixBytes); k, _ = c.Next() {
			return c.Delete()
		}

		return fmt.Errorf("no commit found starting with '%s'", hashPrefix)
	})
}

// Removes all commits older than the provided time
func (l *Ledger) PruneHistoryOlderThan(cutoff time.Time) (int, error) {
	deletedCount := 0

	err := l.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("Ledger"))
		if bucket == nil {
			return nil
		}

		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var commit Commit
			if err := json.Unmarshal(v, &commit); err != nil {
				continue
			}
			commitTime, err := time.Parse(time.RFC3339, commit.Timestamp)
			if err == nil && commitTime.Before(cutoff) {
				if err := c.Delete(); err != nil {
					return err
				}
				deletedCount++
			}
		}
		return nil
	})

	return deletedCount, err
}

// Wipes the visual commit ledger but keeps the cache memory intact
func (l *Ledger) ClearLedgerOnly() error {
	return l.db.Update(func(tx *bbolt.Tx) error {
		//visual Ledger bucket
		err := tx.DeleteBucket([]byte("Ledger"))
		if err != nil && err != bbolt.ErrBucketNotFound {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte("Ledger"))
		return err
	})
}

// Wipes the engine's internal memory of cached layers
func (l *Ledger) ClearCacheMemory() error {
	return l.db.Update(func(tx *bbolt.Tx) error {
		err := tx.DeleteBucket([]byte("KnownLayers"))
		if err != nil && err != bbolt.ErrBucketNotFound {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte("KnownLayers"))

		return err
	})
}

// FailPendingTransfers changes any 'Pending' status commits to 'Failed'
func (l *Ledger) FailPendingTransfers() error {
	return l.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("Ledger"))
		if bucket == nil {
			return nil
		}

		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var commit Commit
			if err := json.Unmarshal(v, &commit); err != nil {
				continue
			}
			if commit.Status == "Pending" {
				commit.Status = "Failed"
				commitJSON, err := json.Marshal(commit)
				if err == nil {
					bucket.Put(k, commitJSON)
				}
			}
		}
		return nil
	})
}
