package updater

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input    string
		expected []int
	}{
		{"1.0.0", []int{1, 0, 0}},
		{"v1.0.0", []int{1, 0, 0}},
		{"2.10.5", []int{2, 10, 5}},
		{"0.0.1", []int{0, 0, 1}},
		{"latest", nil},
		{"v1.0", nil},
		{"1.0.0-rc1", nil},
		{"invalid", nil},
	}

	for _, tc := range tests {
		result := parseSemver(tc.input)
		if tc.expected == nil {
			if result != nil {
				t.Errorf("expected nil for %q, got %v", tc.input, result)
			}
		} else {
			if result == nil || len(result) != 3 {
				t.Fatalf("expected %v for %q, got %v", tc.expected, tc.input, result)
			}
			for i := range tc.expected {
				if result[i] != tc.expected[i] {
					t.Errorf("expected %v for %q, got %v", tc.expected, tc.input, result)
					break
				}
			}
		}
	}
}

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		current  string
		target   string
		expected bool
	}{
		{"1.0.0", "1.0.1", true},
		{"1.0.1", "1.0.0", false},
		{"1.0.0", "1.1.0", true},
		{"1.1.0", "1.0.0", false},
		{"1.0.0", "2.0.0", true},
		{"2.0.0", "1.0.0", false},
		{"1.0.0", "1.0.0", false},
		{"v1.0.0", "1.0.1", true},
		{"1.0.0", "v1.0.1", true},
		{"1.5.9", "1.5.10", true},
		{"invalid", "1.0.0", false},
		{"1.0.0", "invalid", false},
	}

	for _, tc := range tests {
		result := isNewerVersion(tc.current, tc.target)
		if result != tc.expected {
			t.Errorf("isNewerVersion(%q, %q): expected %v, got %v", tc.current, tc.target, tc.expected, result)
		}
	}
}

func TestCheckForUpdate(t *testing.T) {
	// Mock the file system by changing HOME
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	_ = os.MkdirAll(filepath.Join(tmpDir, ".baleen"), 0755)

	// Mock Docker Hub API
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"results": [{"name": "latest"}, {"name": "1.0.5"}, {"name": "1.0.1"}]}`)
	}))
	defer server.Close()

	// Override global var in package
	oldAPI := dockerHubTagsAPI
	dockerHubTagsAPI = server.URL
	defer func() { dockerHubTagsAPI = oldAPI }()

	// Test 1: Initial call should hit network and find update
	res, err := CheckForUpdate("1.0.0", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.UpdateAvailable || res.LatestVersion != "1.0.5" {
		t.Errorf("expected update to 1.0.5, got %v", res)
	}
	if callCount != 1 {
		t.Errorf("expected 1 network call, got %d", callCount)
	}

	// Test 2: Second call without force should use cache
	res2, err := CheckForUpdate("1.0.0", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res2.UpdateAvailable || res2.LatestVersion != "1.0.5" {
		t.Errorf("expected cached update to 1.0.5, got %v", res2)
	}
	if callCount != 1 {
		t.Errorf("expected 1 network call due to cache, got %d", callCount)
	}

	// Test 3: Third call with force should hit network again
	res3, err := CheckForUpdate("1.0.0", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res3.UpdateAvailable || res3.LatestVersion != "1.0.5" {
		t.Errorf("expected forced update to 1.0.5, got %v", res3)
	}
	if callCount != 2 {
		t.Errorf("expected 2 network calls due to force=true, got %d", callCount)
	}
}
