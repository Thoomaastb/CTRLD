package metrics

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// infoCache hält die gecachte SystemInfo.
var (
	cachedInfo     *SystemInfo
	cachedInfoOnce sync.Once
	cachedInfoMu   sync.RWMutex
)

// CollectSystemInfo sammelt statische Inventarisierung.
// Wird einmalig beim ersten Aufruf gesammelt und danach gecacht.
// Docker-Info wird bei jedem Aufruf aktualisiert (Container können sich ändern).
func CollectSystemInfo(ctx context.Context) (*SystemInfo, error) {
	cachedInfoOnce.Do(func() {
		info := &SystemInfo{CollectedAt: time.Now()}

		info.Hostname, _ = os.Hostname()
		info.Architecture = runtime.GOARCH
		info.OS = readOSRelease()
		info.KernelVersion = readKernelVersion()
		info.CPUModel, info.CPUCores = readCPUInfo()

		if ram, err := collectRAM(); err == nil {
			info.RAMTotalBytes = ram.TotalBytes
		}

		cachedInfoMu.Lock()
		cachedInfo = info
		cachedInfoMu.Unlock()
	})

	cachedInfoMu.RLock()
	base := *cachedInfo
	cachedInfoMu.RUnlock()

	// Docker-Info immer frisch (Container-Status ändert sich)
	base.Docker = collectDockerInfo(ctx)

	return &base, nil
}

// ── OS + Kernel ───────────────────────────────────────────────────────────────

func readOSRelease() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return runtime.GOOS
	}
	defer f.Close()

	vals := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		vals[parts[0]] = strings.Trim(parts[1], `"`)
	}

	name := vals["PRETTY_NAME"]
	if name == "" {
		name = vals["NAME"] + " " + vals["VERSION_ID"]
	}
	return strings.TrimSpace(name)
}

func readKernelVersion() string {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return "unknown"
	}
	// Format: "Linux version 5.15.0-91-generic ..."
	fields := strings.Fields(string(data))
	if len(fields) >= 3 {
		return fields[2]
	}
	return strings.TrimSpace(string(data))
}

// ── CPU-Modell ────────────────────────────────────────────────────────────────

func readCPUInfo() (model string, cores int) {
	cores = runtime.NumCPU()

	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return "unknown", cores
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), cores
			}
		}
	}
	return "unknown", cores
}

// ── Docker-Integration ────────────────────────────────────────────────────────

// dockerEndpoints gibt alle möglichen Docker-Endpoints in Priorität zurück.
func dockerEndpoints() []string {
	var endpoints []string

	// 1. DOCKER_HOST Umgebungsvariable (höchste Priorität — deckt Proxy + Custom)
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		endpoints = append(endpoints, host)
	}

	// 2. Standard Unix-Socket
	endpoints = append(endpoints, "unix:///var/run/docker.sock")

	// 3. Rootless Docker
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		endpoints = append(endpoints, fmt.Sprintf("unix://%s/docker.sock", xdg))
	}
	if home := os.Getenv("HOME"); home != "" {
		endpoints = append(endpoints, fmt.Sprintf("unix://%s/.docker/run/docker.sock", home))
	}

	return endpoints
}

// newDockerHTTPClient erstellt einen HTTP-Client für einen Docker-Endpoint.
func newDockerHTTPClient(endpoint string) (*http.Client, string, error) {
	if strings.HasPrefix(endpoint, "unix://") {
		socketPath := strings.TrimPrefix(endpoint, "unix://")
		transport := &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
			},
		}
		return &http.Client{Transport: transport, Timeout: 3 * time.Second},
			"http://localhost", nil
	}

	if strings.HasPrefix(endpoint, "tcp://") || strings.HasPrefix(endpoint, "http://") {
		baseURL := strings.Replace(endpoint, "tcp://", "http://", 1)
		return &http.Client{Timeout: 3 * time.Second}, baseURL, nil
	}

	return nil, "", fmt.Errorf("docker: endpoint-format nicht unterstützt: %s", endpoint)
}

// collectDockerInfo versucht Docker-Informationen zu sammeln.
// Gibt nil zurück wenn Docker nicht verfügbar ist (kein Fehler).
func collectDockerInfo(ctx context.Context) *DockerInfo {
	for _, endpoint := range dockerEndpoints() {
		client, baseURL, err := newDockerHTTPClient(endpoint)
		if err != nil {
			continue
		}

		info := tryDockerEndpoint(ctx, client, baseURL, endpoint)
		if info != nil {
			return info
		}
	}
	return nil
}

// tryDockerEndpoint versucht Docker-API an einem Endpoint zu erreichen.
func tryDockerEndpoint(ctx context.Context, client *http.Client, baseURL, endpoint string) *DockerInfo {
	// /version prüfen
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/version", nil)
	if err != nil {
		return nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var versionResp struct {
		Version string `json:"Version"`
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	_ = json.Unmarshal(body, &versionResp)

	info := &DockerInfo{
		Available: true,
		Version:   versionResp.Version,
		Endpoint:  sanitizeEndpoint(endpoint),
	}

	// Container-Liste holen
	info.Containers = fetchContainers(ctx, client, baseURL)

	return info
}

// fetchContainers lädt die Container-Liste von der Docker-API.
func fetchContainers(ctx context.Context, client *http.Client, baseURL string) []ContainerInfo {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		baseURL+"/containers/json?all=true", nil)
	if err != nil {
		return nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var raw []struct {
		ID    string   `json:"Id"`
		Names []string `json:"Names"`
		Image string   `json:"Image"`
		State string   `json:"State"`
		Status string  `json:"Status"`
		Ports []struct {
			IP          string `json:"IP"`
			PrivatePort int    `json:"PrivatePort"`
			PublicPort  int    `json:"PublicPort"`
			Type        string `json:"Type"`
		} `json:"Ports"`
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}

	containers := make([]ContainerInfo, 0, len(raw))
	for _, c := range raw {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		var ports []string
		for _, p := range c.Ports {
			if p.PublicPort > 0 {
				ports = append(ports, fmt.Sprintf("%d:%d/%s", p.PublicPort, p.PrivatePort, p.Type))
			}
		}

		containers = append(containers, ContainerInfo{
			ID:     c.ID[:min(12, len(c.ID))],
			Name:   name,
			Image:  c.Image,
			State:  c.State,
			Status: c.Status,
			Ports:  ports,
		})
	}
	return containers
}

// sanitizeEndpoint entfernt sensitive Informationen aus dem Endpoint-String.
func sanitizeEndpoint(endpoint string) string {
	// SSH-Credentials entfernen falls vorhanden
	if strings.HasPrefix(endpoint, "ssh://") {
		at := strings.LastIndex(endpoint, "@")
		if at > 0 {
			return "ssh://***@" + endpoint[at+1:]
		}
	}
	return endpoint
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
