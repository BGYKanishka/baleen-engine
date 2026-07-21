package config

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// generate random name
func GenerateNodeName() string {
	adjectives := []string{"Aqua", "Swift", "Deep", "Sonic", "Lunar", "Mighty"}
	nouns := []string{"Whale", "Orca", "Dolphin", "Ray", "Shark", "Baleen"}

	// Seed the random number generator
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	return fmt.Sprintf("%s-%s-%d",
		adjectives[rng.Intn(len(adjectives))],
		nouns[rng.Intn(len(nouns))],
		rng.Intn(1000))
}

// SaveNodeName persists the node name to ~/.baleen/node_name
func SaveNodeName(name string) error {
	baleenRoot, err := BaleenDir()
	if err != nil {
		return err
	}
	path := filepath.Join(baleenRoot, "node_name")
	return os.WriteFile(path, []byte(strings.TrimSpace(name)), 0644)
}

// LoadNodeName returns the saved custom node name, or "" if none is set.
func LoadNodeName() string {
	baleenRoot, err := BaleenDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(baleenRoot, "node_name"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
