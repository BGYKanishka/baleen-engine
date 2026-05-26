package ledger

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"sort"

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

// streams the file into a hash
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

// saves the metadata
func RecordCommit(dbPath string, commit Commit) error {
	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		return err
	}
	defer db.Close()

	return db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("Ledger"))
		if err != nil {
			return err
		}
		commitJSON, err := json.Marshal(commit)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(commit.Hash), commitJSON)
	})
}

// retrieves all commits from the database and sorts them newest first
func GetHistory(dbPath string) ([]Commit, error) {
	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var history []Commit

	err = db.View(func(tx *bbolt.Tx) error {
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

func MarkLayersAsOwned(dbPath string, layers []string) error {
	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		return err
	}
	defer db.Close()

	return db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("KnownLayers"))
		if err != nil {
			return err
		}
		for _, layer := range layers {
			if err := bucket.Put([]byte(layer), []byte("1")); err != nil {
				return err
			}
		}
		return nil
	})
}

func HasLayer(dbPath string, layer string) bool {
	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{ReadOnly: true})
	if err != nil {
		return false
	}
	defer db.Close()

	hasLayer := false
	db.View(func(tx *bbolt.Tx) error {
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
