// Package updater prüft auf neue CTRLD-Releases via GitHub API.
package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/Thoomaastb/CTRLD/pkg/version"
)

const (
	githubAPI      = "https://api.github.com/repos/Thoomaastb/CTRLD/releases/latest"
	checkInterval  = 24 * time.Hour
)

// ReleaseInfo enthält Informationen über ein GitHub-Release.
type ReleaseInfo struct {
	Version     string    `json:"version"`
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
	Body        string    `json:"body"`
	IsPrerelease bool     `json:"is_prerelease"`
}

// UpdateStatus enthält den aktuellen Update-Status.
type UpdateStatus struct {
	CurrentVersion string       `json:"current_version"`
	LatestVersion  string       `json:"latest_version"`
	HasUpdate      bool         `json:"has_update"`
	LastChecked    time.Time    `json:"last_checked"`
	Release        *ReleaseInfo `json:"release,omitempty"`
}

// Checker prüft periodisch auf Updates.
type Checker struct {
	status UpdateStatus
	log    zerolog.Logger
	client *http.Client
}

// NewChecker erstellt einen neuen Update-Checker.
func NewChecker(log zerolog.Logger) *Checker {
	return &Checker{
		log: log,
		client: &http.Client{Timeout: 10 * time.Second},
		status: UpdateStatus{
			CurrentVersion: version.Version,
		},
	}
}

// Start startet den periodischen Update-Check.
func (c *Checker) Start(ctx context.Context) {
	// Erster Check beim Start
	c.check(ctx)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.check(ctx)
		}
	}
}

// Status gibt den aktuellen Update-Status zurück.
func (c *Checker) Status() UpdateStatus {
	return c.status
}

// check führt einen Update-Check durch.
func (c *Checker) check(ctx context.Context) {
	release, err := c.fetchLatestRelease(ctx)
	if err != nil {
		c.log.Debug().Err(err).Msg("update check fehlgeschlagen")
		return
	}

	c.status.LatestVersion = release.Version
	c.status.LastChecked = time.Now()
	c.status.Release = release
	c.status.HasUpdate = isNewerVersion(release.Version, version.Version)

	if c.status.HasUpdate {
		c.log.Info().
			Str("current", version.Version).
			Str("latest", release.Version).
			Msg("neue CTRLD-Version verfügbar")
	}
}

// fetchLatestRelease holt Release-Informationen von GitHub.
func (c *Checker) fetchLatestRelease(ctx context.Context) (*ReleaseInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubAPI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "CTRLD/"+version.Version)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("updater: github anfrage fehlgeschlagen: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("updater: github status %d", resp.StatusCode)
	}

	var raw struct {
		TagName     string    `json:"tag_name"`
		HTMLURL     string    `json:"html_url"`
		PublishedAt time.Time `json:"published_at"`
		Body        string    `json:"body"`
		Prerelease  bool      `json:"prerelease"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("updater: json parsen fehlgeschlagen: %w", err)
	}

	return &ReleaseInfo{
		Version:      raw.TagName,
		URL:          raw.HTMLURL,
		PublishedAt:  raw.PublishedAt,
		Body:         raw.Body,
		IsPrerelease: raw.Prerelease,
	}, nil
}

// isNewerVersion vergleicht zwei Semantic-Versions.
// Gibt true zurück wenn latest > current.
func isNewerVersion(latest, current string) bool {
	// v-Prefix entfernen
	l := strings.TrimPrefix(latest, "v")
	c := strings.TrimPrefix(current, "v")

	// dev/unversioned
	if c == "dev" || c == "" || c == "0.0.0" {
		return false
	}

	lParts := strings.Split(l, ".")
	cParts := strings.Split(c, ".")

	maxLen := len(lParts)
	if len(cParts) > maxLen {
		maxLen = len(cParts)
	}

	for i := 0; i < maxLen; i++ {
		lNum := partToInt(lParts, i)
		cNum := partToInt(cParts, i)
		if lNum > cNum {
			return true
		}
		if lNum < cNum {
			return false
		}
	}
	return false
}

func partToInt(parts []string, i int) int {
	if i >= len(parts) {
		return 0
	}
	n := 0
	for _, c := range parts[i] {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}
