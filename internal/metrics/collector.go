// Package metrics liest System-Metriken von Linux /proc und /sys.
package metrics

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Snapshot enthält alle Metriken zu einem Zeitpunkt.
type Snapshot struct {
	Timestamp time.Time      `json:"timestamp"`
	CPU       CPUMetrics     `json:"cpu"`
	RAM       RAMMetrics     `json:"ram"`
	Disks     []DiskMetrics  `json:"disks"`
	Networks  []NetMetrics   `json:"networks"`
	LoadAvg   LoadAvgMetrics `json:"load_avg"`
	UptimeSec float64        `json:"uptime_sec"`
}

// CPUMetrics enthält CPU-Auslastung gesamt + pro Core.
type CPUMetrics struct {
	UsagePercent     float64   `json:"usage_percent"`
	CoreUsagePercent []float64 `json:"core_usage_percent"`
	NumCores         int       `json:"num_cores"`
}

// RAMMetrics enthält Speicher-Informationen in Bytes.
type RAMMetrics struct {
	TotalBytes     uint64  `json:"total_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	FreeBytes      uint64  `json:"free_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	CachedBytes    uint64  `json:"cached_bytes"`
	BuffersBytes   uint64  `json:"buffers_bytes"`
	SwapTotalBytes uint64  `json:"swap_total_bytes"`
	SwapUsedBytes  uint64  `json:"swap_used_bytes"`
	UsagePercent   float64 `json:"usage_percent"`
}

// DiskMetrics enthält Disk-I/O und Speicherplatz.
type DiskMetrics struct {
	Device       string  `json:"device"`
	MountPoint   string  `json:"mount_point"`
	FSType       string  `json:"fs_type"`
	TotalBytes   uint64  `json:"total_bytes"`
	UsedBytes    uint64  `json:"used_bytes"`
	FreeBytes    uint64  `json:"free_bytes"`
	UsagePercent float64 `json:"usage_percent"`
	ReadBytesPS  float64 `json:"read_bytes_per_sec"`
	WriteBytesPS float64 `json:"write_bytes_per_sec"`
}

// NetMetrics enthält Netzwerk-I/O pro Interface.
type NetMetrics struct {
	Interface    string  `json:"interface"`
	IPAddresses  []string `json:"ip_addresses"`
	MACAddress   string  `json:"mac_address"`
	Type         string  `json:"type"` // physical / loopback / bridge / veth / tun
	LinkState    string  `json:"link_state"` // up / down / unknown
	RxBytesPS    float64 `json:"rx_bytes_per_sec"`
	TxBytesPS    float64 `json:"tx_bytes_per_sec"`
	RxBytesTotal uint64  `json:"rx_bytes_total"`
	TxBytesTotal uint64  `json:"tx_bytes_total"`
}

// LoadAvgMetrics enthält Load Average 1/5/15 Minuten.
type LoadAvgMetrics struct {
	Load1  float64 `json:"load_1"`
	Load5  float64 `json:"load_5"`
	Load15 float64 `json:"load_15"`
}

// Process enthält Informationen zu einem Prozess.
type Process struct {
	PID        int     `json:"pid"`
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpu_percent"`
	MemPercent float64 `json:"mem_percent"`
	MemBytes   uint64  `json:"mem_bytes"`
	Status     string  `json:"status"`
}

// SystemInfo enthält statische System-Inventarisierung.
// Wird einmalig gesammelt und gecacht.
type SystemInfo struct {
	Hostname     string         `json:"hostname"`
	OS           string         `json:"os"`
	KernelVersion string        `json:"kernel_version"`
	Architecture string         `json:"architecture"`
	CPUModel     string         `json:"cpu_model"`
	CPUCores     int            `json:"cpu_cores"`
	RAMTotalBytes uint64        `json:"ram_total_bytes"`
	Docker       *DockerInfo    `json:"docker,omitempty"`
	CollectedAt  time.Time      `json:"collected_at"`
}

// DockerInfo enthält Docker-Inventarisierung (optional).
type DockerInfo struct {
	Available      bool            `json:"available"`
	Version        string          `json:"version,omitempty"`
	Endpoint       string          `json:"endpoint"`
	Containers     []ContainerInfo `json:"containers,omitempty"`
}

// ContainerInfo enthält Infos zu einem Container.
type ContainerInfo struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Image   string   `json:"image"`
	Status  string   `json:"status"`
	State   string   `json:"state"`
	Ports   []string `json:"ports,omitempty"`
}

// cpuStat speichert CPU-Tick-Werte für Delta-Berechnung.
type cpuStat struct {
	user    uint64
	nice    uint64
	system  uint64
	idle    uint64
	iowait  uint64
	irq     uint64
	softirq uint64
}

// diskStat speichert Disk-I/O für Delta-Berechnung.
type diskStat struct {
	readBytes  uint64
	writeBytes uint64
}

// netStat speichert Netzwerk-I/O für Delta-Berechnung.
type netStat struct {
	rxBytes uint64
	txBytes uint64
}

// Collector sammelt System-Metriken.
type Collector struct {
	mu       sync.Mutex
	prevCPU  []cpuStat
	prevDisk map[string]diskStat
	prevNet  map[string]netStat
	prevTime time.Time
}

// NewCollector erstellt einen neuen Collector.
func NewCollector() *Collector {
	c := &Collector{
		prevDisk: make(map[string]diskStat),
		prevNet:  make(map[string]netStat),
		prevTime: time.Now(),
	}
	c.prevCPU, _ = readCPUStat()
	return c
}

// Collect liest alle Metriken und gibt einen Snapshot zurück.
func (c *Collector) Collect(includeProcesses bool) (*Snapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(c.prevTime).Seconds()
	if elapsed < 0.1 {
		elapsed = 0.1
	}

	snap := &Snapshot{Timestamp: now}

	if cpu, err := c.collectCPU(); err == nil {
		snap.CPU = cpu
	}
	if ram, err := collectRAM(); err == nil {
		snap.RAM = ram
	}
	if load, err := collectLoadAvg(); err == nil {
		snap.LoadAvg = load
	}
	if uptime, err := collectUptime(); err == nil {
		snap.UptimeSec = uptime
	}
	if disks, err := c.collectDisks(elapsed); err == nil {
		snap.Disks = disks
	}
	if nets, err := c.collectNetwork(elapsed); err == nil {
		snap.Networks = nets
	}

	c.prevTime = now
	return snap, nil
}

// CollectProcesses sammelt die Prozessliste (teurer Aufruf).
func (c *Collector) CollectProcesses() ([]Process, error) {
	ram, err := collectRAM()
	if err != nil {
		return nil, err
	}
	return collectProcesses(ram.TotalBytes)
}

// ── CPU ───────────────────────────────────────────────────────────────────────

func (c *Collector) collectCPU() (CPUMetrics, error) {
	current, err := readCPUStat()
	if err != nil {
		return CPUMetrics{}, err
	}

	numCores := runtime.NumCPU()
	metrics := CPUMetrics{NumCores: numCores}

	if len(current) == 0 || len(c.prevCPU) == 0 {
		c.prevCPU = current
		return metrics, nil
	}

	if len(c.prevCPU) > 0 {
		metrics.UsagePercent = cpuUsagePercent(c.prevCPU[0], current[0])
	}

	coreCount := len(current) - 1
	if coreCount < 0 {
		coreCount = 0
	}
	metrics.CoreUsagePercent = make([]float64, coreCount)
	for i := 0; i < coreCount && i+1 < len(current) && i+1 < len(c.prevCPU); i++ {
		metrics.CoreUsagePercent[i] = cpuUsagePercent(c.prevCPU[i+1], current[i+1])
	}

	c.prevCPU = current
	return metrics, nil
}

func readCPUStat() ([]cpuStat, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var stats []cpuStat
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu") {
			break
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}
		var s cpuStat
		s.user, _    = parseUint(fields[1])
		s.nice, _    = parseUint(fields[2])
		s.system, _  = parseUint(fields[3])
		s.idle, _    = parseUint(fields[4])
		s.iowait, _  = parseUint(fields[5])
		s.irq, _     = parseUint(fields[6])
		s.softirq, _ = parseUint(fields[7])
		stats = append(stats, s)
	}
	return stats, scanner.Err()
}

func cpuUsagePercent(prev, curr cpuStat) float64 {
	prevTotal := prev.user + prev.nice + prev.system + prev.idle + prev.iowait + prev.irq + prev.softirq
	currTotal := curr.user + curr.nice + curr.system + curr.idle + curr.iowait + curr.irq + curr.softirq
	prevIdle  := prev.idle + prev.iowait
	currIdle  := curr.idle + curr.iowait

	totalDelta := float64(currTotal - prevTotal)
	idleDelta  := float64(currIdle - prevIdle)

	if totalDelta == 0 {
		return 0
	}
	return (1.0 - idleDelta/totalDelta) * 100.0
}

// ── RAM ───────────────────────────────────────────────────────────────────────

func collectRAM() (RAMMetrics, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return RAMMetrics{}, err
	}
	defer f.Close()

	mem := make(map[string]uint64)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		val, _ := parseUint(fields[1])
		mem[key] = val * 1024
	}

	m := RAMMetrics{
		TotalBytes:     mem["MemTotal"],
		FreeBytes:      mem["MemFree"],
		AvailableBytes: mem["MemAvailable"],
		CachedBytes:    mem["Cached"] + mem["SReclaimable"],
		BuffersBytes:   mem["Buffers"],
		SwapTotalBytes: mem["SwapTotal"],
		SwapUsedBytes:  mem["SwapTotal"] - mem["SwapFree"],
	}
	m.UsedBytes = m.TotalBytes - m.AvailableBytes
	if m.TotalBytes > 0 {
		m.UsagePercent = float64(m.UsedBytes) / float64(m.TotalBytes) * 100.0
	}
	return m, scanner.Err()
}

// ── Load Average ──────────────────────────────────────────────────────────────

func collectLoadAvg() (LoadAvgMetrics, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return LoadAvgMetrics{}, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return LoadAvgMetrics{}, fmt.Errorf("ungültiges loadavg format")
	}
	var l LoadAvgMetrics
	l.Load1, _  = strconv.ParseFloat(fields[0], 64)
	l.Load5, _  = strconv.ParseFloat(fields[1], 64)
	l.Load15, _ = strconv.ParseFloat(fields[2], 64)
	return l, nil
}

// ── Uptime ────────────────────────────────────────────────────────────────────

func collectUptime() (float64, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0, fmt.Errorf("ungültiges uptime format")
	}
	return strconv.ParseFloat(fields[0], 64)
}

// ── Disk ──────────────────────────────────────────────────────────────────────

func (c *Collector) collectDisks(elapsed float64) ([]DiskMetrics, error) {
	ioStats, err := readDiskStats()
	if err != nil {
		return nil, err
	}

	mounts, err := readMounts()
	if err != nil {
		return nil, err
	}

	var disks []DiskMetrics
	seen := make(map[string]bool)

	for _, mount := range mounts {
		if seen[mount.device] || !isRealFS(mount.fstype) {
			continue
		}

		var stat syscallStatfs
		if err := statfs(mount.point, &stat); err != nil {
			continue
		}
		if stat.Bsize <= 0 {
			continue
		}

		d := DiskMetrics{
			Device:     mount.device,
			MountPoint: mount.point,
			FSType:     mount.fstype,
			TotalBytes: stat.Blocks * uint64(stat.Bsize),
			FreeBytes:  stat.Bfree * uint64(stat.Bsize),
		}
		d.UsedBytes = d.TotalBytes - d.FreeBytes
		if d.TotalBytes > 0 {
			d.UsagePercent = float64(d.UsedBytes) / float64(d.TotalBytes) * 100.0
		}

		devName := filepath.Base(mount.device)
		if io, ok := ioStats[devName]; ok {
			if prev, ok := c.prevDisk[devName]; ok {
				d.ReadBytesPS  = float64(io.readBytes-prev.readBytes) / elapsed
				d.WriteBytesPS = float64(io.writeBytes-prev.writeBytes) / elapsed
			}
			c.prevDisk[devName] = diskStat{readBytes: io.readBytes, writeBytes: io.writeBytes}
		}

		seen[mount.device] = true
		disks = append(disks, d)
	}
	return disks, nil
}

type mountEntry struct {
	device string
	point  string
	fstype string
}

func readMounts() ([]mountEntry, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var mounts []mountEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		mounts = append(mounts, mountEntry{device: fields[0], point: fields[1], fstype: fields[2]})
	}
	return mounts, scanner.Err()
}

func isRealFS(fstype string) bool {
	realFS := map[string]bool{
		"ext4": true, "ext3": true, "ext2": true,
		"xfs": true, "btrfs": true, "zfs": true,
		"vfat": true, "ntfs": true, "exfat": true,
		"tmpfs": true, "overlay": true,
	}
	return realFS[fstype]
}

type rawDiskIO struct {
	readBytes  uint64
	writeBytes uint64
}

func readDiskStats() (map[string]rawDiskIO, error) {
	f, err := os.Open("/proc/diskstats")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	stats := make(map[string]rawDiskIO)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}
		name := fields[2]
		readSectors, _  := parseUint(fields[5])
		writeSectors, _ := parseUint(fields[9])
		stats[name] = rawDiskIO{readBytes: readSectors * 512, writeBytes: writeSectors * 512}
	}
	return stats, scanner.Err()
}

// ── Netzwerk (vollständig inkl. Docker) ───────────────────────────────────────

func (c *Collector) collectNetwork(elapsed float64) ([]NetMetrics, error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var nets []NetMetrics
	scanner := bufio.NewScanner(f)
	scanner.Scan() // Header 1
	scanner.Scan() // Header 2

	for scanner.Scan() {
		line := scanner.Text()
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		iface := strings.TrimSpace(line[:colonIdx])

		fields := strings.Fields(line[colonIdx+1:])
		if len(fields) < 9 {
			continue
		}

		rxBytes, _ := parseUint(fields[0])
		txBytes, _ := parseUint(fields[8])

		n := NetMetrics{
			Interface:    iface,
			RxBytesTotal: rxBytes,
			TxBytesTotal: txBytes,
			Type:         ifaceType(iface),
			LinkState:    readLinkState(iface),
			MACAddress:   readMAC(iface),
			IPAddresses:  readIPAddresses(iface),
		}

		if prev, ok := c.prevNet[iface]; ok {
			n.RxBytesPS = float64(rxBytes-prev.rxBytes) / elapsed
			n.TxBytesPS = float64(txBytes-prev.txBytes) / elapsed
		}
		c.prevNet[iface] = netStat{rxBytes: rxBytes, txBytes: txBytes}

		nets = append(nets, n)
	}
	return nets, scanner.Err()
}

// ifaceType erkennt den Interface-Typ anhand des Namens.
func ifaceType(iface string) string {
	switch {
	case iface == "lo":
		return "loopback"
	case strings.HasPrefix(iface, "docker"):
		return "bridge"
	case strings.HasPrefix(iface, "br-"):
		return "bridge"
	case strings.HasPrefix(iface, "veth"):
		return "veth"
	case strings.HasPrefix(iface, "tun") || strings.HasPrefix(iface, "tap"):
		return "tun"
	case strings.HasPrefix(iface, "virbr"):
		return "bridge"
	default:
		return "physical"
	}
}

// readLinkState liest den Link-Status aus /sys/class/net/<iface>/operstate.
func readLinkState(iface string) string {
	data, err := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/operstate", iface))
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(data))
}

// readMAC liest die MAC-Adresse aus /sys/class/net/<iface>/address.
func readMAC(iface string) string {
	data, err := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/address", iface))
	if err != nil {
		return ""
	}
	mac := strings.TrimSpace(string(data))
	if mac == "00:00:00:00:00:00" {
		return ""
	}
	return mac
}

// readIPAddresses liest IP-Adressen aus /proc/net/fib_trie (IPv4).
func readIPAddresses(iface string) []string {
	// Einfacherer Ansatz: /proc/net/if_inet6 für IPv6 + eigene Parsing für IPv4
	// Für IPv4: lesen wir aus /proc/net/fib_trie ist komplex — nutze /proc/net/dev + ifconfig-Alternative
	// Einfachste zuverlässige Methode: /sys/class/net/<iface>/
	var ips []string

	// IPv4 via /proc/net/fib_trie — zu komplex, stattdessen via Netlink-ähnlicher Methode
	// Für v0.11.0: IP aus /proc/net/if_inet6 (IPv6) lesen, IPv4 TODO für v0.12.0
	f, err := os.Open("/proc/net/if_inet6")
	if err != nil {
		return ips
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 6 {
			continue
		}
		if fields[5] != iface {
			continue
		}
		// IPv6-Adresse formatieren
		addr := fields[0]
		if len(addr) == 32 {
			var formatted strings.Builder
			for i := 0; i < 32; i += 4 {
				if i > 0 {
					formatted.WriteString(":")
				}
				formatted.WriteString(addr[i : i+4])
			}
			ip := formatted.String()
			// Link-local überspringen
			if !strings.HasPrefix(ip, "fe80") {
				ips = append(ips, ip)
			}
		}
	}
	return ips
}

// ── Prozesse ──────────────────────────────────────────────────────────────────

func collectProcesses(totalRAM uint64) ([]Process, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	var procs []Process
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		p, err := readProcess(pid, totalRAM)
		if err != nil {
			continue
		}
		procs = append(procs, p)
	}
	return procs, nil
}

func readProcess(pid int, totalRAM uint64) (Process, error) {
	commBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return Process{}, err
	}

	statusBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return Process{}, err
	}

	p := Process{
		PID:  pid,
		Name: strings.TrimSpace(string(commBytes)),
	}

	for _, line := range strings.Split(string(statusBytes), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "State":
			p.Status = fields[1]
		case "VmRSS":
			kb, _ := parseUint(fields[1])
			p.MemBytes = kb * 1024
			if totalRAM > 0 {
				p.MemPercent = float64(p.MemBytes) / float64(totalRAM) * 100.0
			}
		}
	}
	return p, nil
}

func parseUint(s string) (uint64, error) {
	return strconv.ParseUint(strings.TrimSpace(s), 10, 64)
}
