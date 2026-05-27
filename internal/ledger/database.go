package ledger

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
