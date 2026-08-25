package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/config"
)

var dockerHubTagsAPI = "https://hub.docker.com/v2/repositories/yehankanishka/baleen-engine/tags"

const cacheLifetime = 6 * time.Hour

type UpdateCheckResult struct {
	UpdateAvailable bool   `json:"update_available"`
	LatestVersion   string `json:"latest_version"`
	CurrentVersion  string `json:"current_version"`
}

type updateCache struct {
	LastChecked   time.Time `json:"last_checked"`
	LatestVersion string    `json:"latest_version"`
}

type dockerHubResponse struct {
	Results []struct {
		Name string `json:"name"`
	} `json:"results"`
}

// checks if a new version is available on Docker Hub.
func CheckForUpdate(currentVersion string, force bool) (UpdateCheckResult, error) {
	cachePath, err := getCachePath()
	if err != nil {
		return UpdateCheckResult{}, err
	}

	var latest string

	// Read cache
	cacheData, err := os.ReadFile(cachePath)
	if err == nil && !force {
		var cache updateCache
		if err := json.Unmarshal(cacheData, &cache); err == nil {
			if time.Since(cache.LastChecked) < cacheLifetime {
				latest = cache.LatestVersion
			}
		}
	}

	// Fetch from Docker Hub if cache missed or expired
	if latest == "" {
		fetched, fetchErr := fetchLatestFromDockerHub()
		if fetchErr != nil {
			// If we fail to fetch, and we had a cached version (even expired), use it
			if err == nil {
				var cache updateCache
				if json.Unmarshal(cacheData, &cache) == nil {
					latest = cache.LatestVersion
				}
			}
			if latest == "" {
				return UpdateCheckResult{}, fmt.Errorf("failed to check for updates: %w", fetchErr)
			}
		} else {
			latest = fetched
			// Write cache
			newCache := updateCache{
				LastChecked:   time.Now(),
				LatestVersion: latest,
			}
			cacheBytes, _ := json.Marshal(newCache)
			_ = os.WriteFile(cachePath, cacheBytes, 0644)
		}
	}

	isUpdate := isNewerVersion(currentVersion, latest)

	return UpdateCheckResult{
		UpdateAvailable: isUpdate,
		LatestVersion:   latest,
		CurrentVersion:  currentVersion,
	}, nil
}

func fetchLatestFromDockerHub() (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(dockerHubTagsAPI)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("docker hub returned status %d", resp.StatusCode)
	}

	var body dockerHubResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}

	var versions []string
	for _, res := range body.Results {
		// Ignore "latest" and any non-semver looking tags
		if res.Name != "latest" && parseSemver(res.Name) != nil {
			versions = append(versions, res.Name)
		}
	}

	if len(versions) == 0 {
		return "", fmt.Errorf("no version tags found")
	}

	// Sort versions descending
	sort.Slice(versions, func(i, j int) bool {
		return isNewerVersion(versions[j], versions[i])
	})

	return versions[0], nil
}

// returns true if target > current
func isNewerVersion(current, target string) bool {
	c := parseSemver(current)
	t := parseSemver(target)
	if c == nil || t == nil {
		return false
	}
	if t[0] != c[0] {
		return t[0] > c[0]
	}
	if t[1] != c[1] {
		return t[1] > c[1]
	}
	return t[2] > c[2]
}

func parseSemver(v string) []int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return nil
	}
	parsed := make([]int, 3)
	for i := 0; i < 3; i++ {
		val, err := strconv.Atoi(parts[i])
		if err != nil {
			return nil
		}
		parsed[i] = val
	}
	return parsed
}

func getCachePath() (string, error) {
	baleenDir, err := config.BaleenDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baleenDir, "update_cache.json"), nil
}
