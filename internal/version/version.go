// Package version provides version information and update checking.
package version

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	// Version is the current version of dap-mcp
	Version = "0.1.1"

	// GitHubRepo is the repository path
	GitHubRepo = "ctagard/dap-mcp"

	// GitHubAPIURL is the GitHub API endpoint for latest release
	GitHubAPIURL = "https://api.github.com/repos/%s/releases/latest"
)

// UpdateInfo contains information about available updates
type UpdateInfo struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	ReleaseURL      string `json:"release_url,omitempty"`
	ReleaseNotes    string `json:"release_notes,omitempty"`
	CheckedAt       time.Time `json:"checked_at"`
	Error           string `json:"error,omitempty"`
}

// UpdateMessage returns a human-readable message about the update
func (u *UpdateInfo) UpdateMessage() string {
	if u.Error != "" {
		return ""
	}
	if !u.UpdateAvailable {
		return ""
	}
	return fmt.Sprintf(
		"A new version of dap-mcp is available: v%s (current: v%s). "+
			"Update with: curl -sSL https://raw.githubusercontent.com/%s/main/scripts/install.sh | bash",
		u.LatestVersion, u.CurrentVersion, GitHubRepo,
	)
}

// Checker handles version checking
type Checker struct {
	mu         sync.RWMutex
	updateInfo *UpdateInfo
	checked    bool
}

// NewChecker creates a new version checker
func NewChecker() *Checker {
	return &Checker{}
}

// githubRelease represents the GitHub API response for a release
type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Body    string `json:"body"`
}

// CheckForUpdates checks GitHub for a newer version
func (c *Checker) CheckForUpdates(ctx context.Context) *UpdateInfo {
	c.mu.Lock()
	defer c.mu.Unlock()

	info := &UpdateInfo{
		CurrentVersion: Version,
		CheckedAt:      time.Now(),
	}

	// Use background context if none provided
	if ctx == nil {
		ctx = context.Background()
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	url := fmt.Sprintf(GitHubAPIURL, GitHubRepo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		info.Error = fmt.Sprintf("failed to create request: %v", err)
		c.updateInfo = info
		c.checked = true
		return info
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "dap-mcp/"+Version)

	resp, err := client.Do(req)
	if err != nil {
		info.Error = fmt.Sprintf("failed to check for updates: %v", err)
		c.updateInfo = info
		c.checked = true
		return info
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		info.Error = fmt.Sprintf("GitHub API returned status %d", resp.StatusCode)
		c.updateInfo = info
		c.checked = true
		return info
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		info.Error = fmt.Sprintf("failed to parse response: %v", err)
		c.updateInfo = info
		c.checked = true
		return info
	}

	// Parse version from tag (remove 'v' prefix if present)
	latestVersion := strings.TrimPrefix(release.TagName, "v")
	info.LatestVersion = latestVersion
	info.ReleaseURL = release.HTMLURL
	info.ReleaseNotes = truncateString(release.Body, 500)
	info.UpdateAvailable = compareVersions(Version, latestVersion) < 0

	c.updateInfo = info
	c.checked = true
	return info
}

// CheckForUpdatesAsync checks for updates in the background
func (c *Checker) CheckForUpdatesAsync() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		c.CheckForUpdates(ctx)
	}()
}

// GetUpdateInfo returns the cached update info
func (c *Checker) GetUpdateInfo() *UpdateInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.updateInfo
}

// HasChecked returns whether an update check has been performed
func (c *Checker) HasChecked() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.checked
}

// compareVersions compares two semver strings
// Returns -1 if v1 < v2, 0 if equal, 1 if v1 > v2
func compareVersions(v1, v2 string) int {
	// Parse version components
	parse := func(v string) (major, minor, patch int) {
		parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
		if len(parts) >= 1 {
			fmt.Sscanf(parts[0], "%d", &major)
		}
		if len(parts) >= 2 {
			fmt.Sscanf(parts[1], "%d", &minor)
		}
		if len(parts) >= 3 {
			// Handle pre-release suffixes like "1.0.0-beta"
			patchStr := strings.Split(parts[2], "-")[0]
			fmt.Sscanf(patchStr, "%d", &patch)
		}
		return
	}

	maj1, min1, pat1 := parse(v1)
	maj2, min2, pat2 := parse(v2)

	if maj1 != maj2 {
		if maj1 < maj2 {
			return -1
		}
		return 1
	}
	if min1 != min2 {
		if min1 < min2 {
			return -1
		}
		return 1
	}
	if pat1 != pat2 {
		if pat1 < pat2 {
			return -1
		}
		return 1
	}
	return 0
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// GetVersion returns the current version
func GetVersion() string {
	return Version
}
