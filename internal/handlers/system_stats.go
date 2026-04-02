package handlers

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/anonvector/slipgate/internal/actions"
	"github.com/anonvector/slipgate/internal/config"
	"github.com/anonvector/slipgate/internal/service"
	"golang.org/x/term"
)

var sparkRunes = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

const graphWidth = 40

func handleSystemStats(ctx *actions.Context) error {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("cannot enter raw mode: %w", err)
	}
	defer term.Restore(fd, oldState)

	fmt.Print("\033[?25l")                    // hide cursor
	defer fmt.Print("\033[?25h\033[H\033[2J") // show cursor + clear on exit

	cpuHist := make([]float64, 0, graphWidth)
	ramHist := make([]float64, 0, graphWidth)
	rxHist := make([]float64, 0, graphWidth)
	txHist := make([]float64, 0, graphWidth)

	// Seed CPU and traffic baselines.
	prevIdle, prevTotal := readCPUStat()
	prevRX, prevTX := interfaceTraffic()

	// Quit on q / Q / Ctrl-C.
	quit := make(chan struct{})
	go func() {
		buf := make([]byte, 1)
		for {
			n, _ := os.Stdin.Read(buf)
			if n > 0 && (buf[0] == 'q' || buf[0] == 'Q' || buf[0] == 3) {
				close(quit)
				return
			}
		}
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	cfg, _ := ctx.Config.(*config.Config)

	// Clear screen and draw initial blank state.
	fmt.Print("\033[H\033[2J")
	drawDashboard(cpuHist, ramHist, rxHist, txHist, 0, 0, 0, 0,
		0, 0, 0, nil, nil)

	for {
		select {
		case <-quit:
			return nil
		case <-ticker.C:
			// CPU delta.
			idle, total := readCPUStat()
			cpuPct := 0.0
			if dt := total - prevTotal; dt > 0 {
				cpuPct = float64(dt-(idle-prevIdle)) / float64(dt) * 100
			}
			prevIdle, prevTotal = idle, total

			// RAM.
			totalMB, usedMB := memoryUsage()
			ramPct := 0.0
			if totalMB > 0 {
				ramPct = float64(usedMB) * 100 / float64(totalMB)
			}

			// Traffic throughput (bytes/sec).
			rx, tx := interfaceTraffic()
			rxRate := float64(0)
			txRate := float64(0)
			if prevRX > 0 && rx >= prevRX {
				rxRate = float64(rx - prevRX)
			}
			if prevTX > 0 && tx >= prevTX {
				txRate = float64(tx - prevTX)
			}
			prevRX, prevTX = rx, tx

			cpuHist = appendCapped(cpuHist, cpuPct)
			ramHist = appendCapped(ramHist, ramPct)
			rxHist = appendCapped(rxHist, rxRate)
			txHist = appendCapped(txHist, txRate)

			sshSessions := countSSHSessions()

			tunnels := activeTunnels(cfg)
			connStats := connectionStats(cfg)

			drawDashboard(cpuHist, ramHist, rxHist, txHist,
				cpuPct, ramPct, rxRate, txRate,
				totalMB, usedMB, sshSessions, tunnels, connStats)
		}
	}
}

// tunnelInfo holds display info for an active tunnel.
type tunnelInfo struct {
	tag       string
	transport string
	backend   string
	status    string
}

// connStat holds per-protocol connection counts.
type connStat struct {
	protocol string // transport name (dnstt, slipstream, vaydns, naive, ssh, socks5)
	ssh      int    // SSH sessions through this protocol
	socks    int    // SOCKS connections through this protocol
}

// connectionStats counts active SSH and SOCKS connections grouped by protocol.
func connectionStats(cfg *config.Config) []connStat {
	if cfg == nil {
		return nil
	}

	// Count SSH tunnel users (slipgate-ssh group members with active sessions).
	sshByUser := countSSHByUser()

	// Count SOCKS connections per backend port.
	socksConns := countSOCKSConnections()

	// Map transport → aggregated counts.
	type counts struct{ ssh, socks int }
	byProto := make(map[string]*counts)

	for _, t := range cfg.Tunnels {
		if !t.Enabled {
			continue
		}
		proto := t.Transport
		if proto == config.TransportDNSTT {
			proto = "dnstt"
		}
		if byProto[proto] == nil {
			byProto[proto] = &counts{}
		}
		if t.Backend == config.BackendSSH {
			// SSH tunnel users connect through SSH on port 22.
			// All tunnel SSH users share the same sshd, so count once per protocol.
			byProto[proto].ssh = len(sshByUser)
		}
		if t.Backend == config.BackendSOCKS {
			backend := cfg.GetBackend(t.Backend)
			if backend != nil {
				byProto[proto].socks = socksConns[backend.Address]
			}
		}
	}

	var stats []connStat
	for proto, c := range byProto {
		if c.ssh > 0 || c.socks > 0 {
			stats = append(stats, connStat{protocol: proto, ssh: c.ssh, socks: c.socks})
		}
	}
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].protocol < stats[j].protocol
	})
	return stats
}

// countSSHByUser returns a map of username → session count for slipgate-ssh users.
func countSSHByUser() map[string]int {
	result := make(map[string]int)

	// Get slipgate-ssh group members.
	members := sshGroupMembers()
	if len(members) == 0 {
		return result
	}
	memberSet := make(map[string]bool, len(members))
	for _, m := range members {
		memberSet[m] = true
	}

	// Parse who output for matching users.
	data, err := exec.Command("who").Output()
	if err != nil {
		return result
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 1 && memberSet[fields[0]] {
			result[fields[0]]++
		}
	}
	return result
}

// sshGroupMembers returns usernames in the slipgate-ssh group.
func sshGroupMembers() []string {
	data, err := os.ReadFile("/etc/group")
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, ":", 4)
		if len(parts) == 4 && parts[0] == config.SSHGroup {
			if parts[3] == "" {
				return nil
			}
			return strings.Split(parts[3], ",")
		}
	}
	return nil
}

// countSOCKSConnections counts established TCP connections per local address
// (e.g. "127.0.0.1:1080" → 5) using /proc/net/tcp.
func countSOCKSConnections() map[string]int {
	result := make(map[string]int)
	data, err := exec.Command("ss", "-tn", "state", "established").Output()
	if err != nil {
		return result
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		local := fields[3]
		// Match known SOCKS backend addresses.
		if strings.HasSuffix(local, ":1080") {
			result[local]++
		}
	}
	return result
}

// activeTunnels returns up to 10 tunnels with their status.
// DNSTT tunnels also generate a noizdns variant row (same service).
func activeTunnels(cfg *config.Config) []tunnelInfo {
	if cfg == nil || len(cfg.Tunnels) == 0 {
		return nil
	}

	var infos []tunnelInfo
	for _, t := range cfg.Tunnels {
		if t.IsDirectTransport() {
			continue
		}
		svcName := service.TunnelServiceName(t.Tag)
		status, err := service.Status(svcName)
		if err != nil {
			status = "unknown"
		}
		infos = append(infos, tunnelInfo{
			tag:       t.Tag,
			transport: t.Transport,
			backend:   t.Backend,
			status:    status,
		})
		// DNSTT serves both dnstt and noizdns clients on the same process.
		if t.Transport == config.TransportDNSTT {
			noizTag := strings.ReplaceAll(t.Tag, "dnstt", "noizdns")
			infos = append(infos, tunnelInfo{
				tag:       noizTag,
				transport: "noizdns",
				backend:   t.Backend,
				status:    status,
			})
		}
	}

	// Sort: active first, then by tag.
	sort.Slice(infos, func(i, j int) bool {
		if infos[i].status == "active" && infos[j].status != "active" {
			return true
		}
		if infos[i].status != "active" && infos[j].status == "active" {
			return false
		}
		return infos[i].tag < infos[j].tag
	})

	if len(infos) > 10 {
		infos = infos[:10]
	}
	return infos
}

// appendCapped appends v to s and trims to graphWidth.
func appendCapped(s []float64, v float64) []float64 {
	s = append(s, v)
	if len(s) > graphWidth {
		s = s[len(s)-graphWidth:]
	}
	return s
}

func drawDashboard(cpuH, ramH, rxH, txH []float64,
	cpuPct, ramPct, rxRate, txRate float64,
	totalMB, usedMB uint64, sshSessions int, tunnels []tunnelInfo, connStats []connStat) {

	var b strings.Builder
	b.WriteString("\033[H") // cursor home

	b.WriteString("\r\n")
	b.WriteString("  \033[1mSlipGate Live Stats\033[0m\r\n")
	b.WriteString("  ─────────────────────────────────────────────────────\r\n\r\n")

	// CPU + RAM sparklines.
	b.WriteString(fmt.Sprintf("  \033[1mCPU\033[0m  %5.1f%%  %s\r\n", cpuPct, sparkline(cpuH, 100, "\033[36m")))
	b.WriteString(fmt.Sprintf("  \033[1mRAM\033[0m  %5.1f%%  %s\r\n\r\n", ramPct, sparkline(ramH, 100, "\033[35m")))

	// RAM bar.
	b.WriteString(fmt.Sprintf("  RAM  %s  %d / %d MB\r\n\r\n", progressBar(ramPct), usedMB, totalMB))

	// Traffic throughput sparklines.
	rxMax := autoMax(rxH)
	txMax := autoMax(txH)
	b.WriteString(fmt.Sprintf("  \033[1m↓\033[0m %9s/s  %s\r\n", formatBytes(uint64(rxRate)), sparkline(rxH, rxMax, "\033[32m")))
	b.WriteString(fmt.Sprintf("  \033[1m↑\033[0m %9s/s  %s\r\n\r\n", formatBytes(uint64(txRate)), sparkline(txH, txMax, "\033[33m")))

	// SSH sessions.
	b.WriteString(fmt.Sprintf("  \033[1mSSH Sessions:\033[0m %d\r\n\r\n", sshSessions))

	// Connected users by protocol.
	b.WriteString("  \033[1mConnections\033[0m\r\n")
	b.WriteString("  ───────────\r\n")
	if len(connStats) == 0 {
		b.WriteString("  (no active connections)\r\n")
	} else {
		b.WriteString(fmt.Sprintf("  %-15s %6s %7s %7s\r\n",
			"\033[2mPROTOCOL\033[0m", "\033[2mSSH\033[0m", "\033[2mSOCKS\033[0m", "\033[2mTOTAL\033[0m"))
		totalSSH, totalSOCKS := 0, 0
		for _, cs := range connStats {
			total := cs.ssh + cs.socks
			b.WriteString(fmt.Sprintf("  %-15s %6d %7d %7d\r\n",
				cs.protocol, cs.ssh, cs.socks, total))
			totalSSH += cs.ssh
			totalSOCKS += cs.socks
		}
		if len(connStats) > 1 {
			b.WriteString(fmt.Sprintf("  \033[2m%-15s %6d %7d %7d\033[0m\r\n",
				"total", totalSSH, totalSOCKS, totalSSH+totalSOCKS))
		}
	}
	b.WriteString("\r\n")

	// Active tunnels.
	b.WriteString("  \033[1mTunnels\033[0m\r\n")
	b.WriteString("  ───────\r\n")
	if len(tunnels) == 0 {
		b.WriteString("  (none configured)\r\n")
	} else {
		b.WriteString(fmt.Sprintf("  %-18s %-13s %-8s %s\r\n",
			"\033[2mTAG\033[0m", "\033[2mTYPE\033[0m", "\033[2mBACKEND\033[0m", "\033[2mSTATUS\033[0m"))
		for _, t := range tunnels {
			dot := "\033[31m●\033[0m" // red
			if t.status == "active" {
				dot = "\033[32m●\033[0m" // green
			}
			b.WriteString(fmt.Sprintf("  %-18s %-13s %-8s %s %s\r\n",
				t.tag, t.transport, t.backend, dot, t.status))
		}
	}

	b.WriteString("\r\n  \033[2mPress Ctrl+C to exit\033[0m\r\n")

	b.WriteString("\033[J") // clear to end of screen

	fmt.Print(b.String())
}

// autoMax returns the max value in data, with a minimum floor of 1024 (1 KB/s)
// to avoid flat-lining the sparkline on idle traffic.
func autoMax(data []float64) float64 {
	m := 1024.0
	for _, v := range data {
		if v > m {
			m = v
		}
	}
	return m
}

func sparkline(data []float64, maxVal float64, color string) string {
	var b strings.Builder
	b.WriteString(color)
	pad := graphWidth - len(data)
	for i := 0; i < pad; i++ {
		b.WriteRune(sparkRunes[0])
	}
	for _, v := range data {
		idx := int(v / maxVal * float64(len(sparkRunes)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkRunes) {
			idx = len(sparkRunes) - 1
		}
		b.WriteRune(sparkRunes[idx])
	}
	b.WriteString("\033[0m")
	return b.String()
}

func progressBar(pct float64) string {
	const width = 40
	filled := int(pct / 100 * width)
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	var b strings.Builder
	b.WriteString("\033[32m")
	for i := 0; i < filled; i++ {
		b.WriteRune('█')
	}
	b.WriteString("\033[0m")
	for i := filled; i < width; i++ {
		b.WriteRune('░')
	}
	return b.String()
}

// readCPUStat reads the aggregate CPU line from /proc/stat and returns
// (idle, total) counters.
func readCPUStat() (idle, total uint64) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return 0, 0
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 {
		return 0, 0
	}
	var vals [10]uint64
	for i := 1; i < len(fields) && i <= 10; i++ {
		fmt.Sscanf(fields[i], "%d", &vals[i-1])
	}
	for _, v := range vals {
		total += v
	}
	idle = vals[3]
	return idle, total
}

// ---------- shared helpers ----------

// countSSHSessions counts active SSH sessions via who(1).
// Each line in who output represents one logged-in session.
func countSSHSessions() int {
	data, err := exec.Command("who").Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return 0
	}
	return len(lines)
}

func interfaceTraffic() (uint64, uint64) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		iface := strings.TrimSpace(line[:idx])
		if iface == "lo" {
			continue
		}
		fields := strings.Fields(line[idx+1:])
		if len(fields) < 10 {
			continue
		}
		var rx, tx uint64
		fmt.Sscanf(fields[0], "%d", &rx)
		fmt.Sscanf(fields[8], "%d", &tx)
		return rx, tx
	}
	return 0, 0
}

func memoryUsage() (uint64, uint64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	var total, available uint64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			fmt.Sscanf(line, "MemTotal: %d kB", &total)
		case strings.HasPrefix(line, "MemAvailable:"):
			fmt.Sscanf(line, "MemAvailable: %d kB", &available)
		}
	}
	totalMB := total / 1024
	usedMB := (total - available) / 1024
	return totalMB, usedMB
}

func formatBytes(b uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case b >= TB:
		return fmt.Sprintf("%.2f TB", float64(b)/float64(TB))
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.2f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
