package config

import (
	"fmt"
	"math/rand"
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
