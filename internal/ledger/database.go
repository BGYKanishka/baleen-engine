package ledger

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"

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

//streams the file into a hash
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