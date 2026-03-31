package context

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/free5gc/nwdaf/internal/logger"
)

// NfProcMetrics holds a sampled snapshot of a single NF's process metrics.
type NfProcMetrics struct {
	PID        int
	CPUPercent float64 // system-relative CPU%, 0–100
	MemRSSKB   uint64  // resident set size in kB
}

// MemLoadPercent returns memory usage as a percentage of refMB megabytes.
func (m *NfProcMetrics) MemLoadPercent(refMB uint64) int32 {
	if refMB == 0 {
		return 0
	}
	pct := m.MemRSSKB * 100 / (refMB * 1024)
	if pct > 100 {
		pct = 100
	}
	return int32(pct)
}

// prevCPUSample stores the values needed to compute a CPU% delta between two samples.
type prevCPUSample struct {
	procTicks  uint64
	totalTicks uint64
}

// ProcMetricsCollector samples /proc for each known NF at a fixed interval
// and caches the results. A background goroutine is started by NewProcMetricsCollector.
type ProcMetricsCollector struct {
	mu          sync.RWMutex
	metrics     map[string]*NfProcMetrics // keyed by lower-case NF name
	prevSamples map[string]prevCPUSample
	stopCh      chan struct{}
}

// knownNFs is the set of NF binary names to track.
var knownNFs = []string{
	"nrf", "amf", "smf", "upf", "udr", "udm",
	"ausf", "pcf", "nssf", "chf", "nwdaf",
}

// NewProcMetricsCollector creates a collector and starts its background sampling
// goroutine. Call Stop() to shut it down.
func NewProcMetricsCollector(interval time.Duration) *ProcMetricsCollector {
	c := &ProcMetricsCollector{
		metrics:     make(map[string]*NfProcMetrics),
		prevSamples: make(map[string]prevCPUSample),
		stopCh:      make(chan struct{}),
	}
	go c.run(interval)
	return c
}

// Stop shuts down the background goroutine.
func (c *ProcMetricsCollector) Stop() {
	close(c.stopCh)
}

// Get returns the latest cached metrics for nfName (case-insensitive).
// Returns nil if the NF process was not found in the last sample.
func (c *ProcMetricsCollector) Get(nfName string) *NfProcMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.metrics[strings.ToLower(nfName)]
}

func (c *ProcMetricsCollector) run(interval time.Duration) {
	// First pass: record baseline ticks only (cannot compute delta yet).
	c.sample(false)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.sample(true)
		case <-c.stopCh:
			return
		}
	}
}

func (c *ProcMetricsCollector) sample(computeCPU bool) {
	totalTicks, err := readTotalCPUTicks()
	if err != nil {
		logger.CtxLog.Warnf("[ProcMetrics] failed to read /proc/stat: %v", err)
		return
	}

	newMetrics := make(map[string]*NfProcMetrics, len(knownNFs))
	newPrevSamples := make(map[string]prevCPUSample, len(knownNFs))

	c.mu.RLock()
	prevSamples := c.prevSamples
	c.mu.RUnlock()

	for _, name := range knownNFs {
		pid, err := findPIDByComm(name)
		if err != nil {
			continue // NF not running
		}

		procTicks, err := readProcCPUTicks(pid)
		if err != nil {
			logger.CtxLog.Debugf("[ProcMetrics] %s: failed to read CPU ticks: %v", name, err)
			continue
		}

		memRSS, err := readProcMemRSSKB(pid)
		if err != nil {
			logger.CtxLog.Debugf("[ProcMetrics] %s: failed to read memory: %v", name, err)
			continue
		}

		m := &NfProcMetrics{PID: pid, MemRSSKB: memRSS}

		if computeCPU {
			if prev, ok := prevSamples[name]; ok && totalTicks > prev.totalTicks {
				deltaTot := totalTicks - prev.totalTicks
				var deltaProc uint64
				if procTicks >= prev.procTicks {
					deltaProc = procTicks - prev.procTicks
				}
				pct := float64(deltaProc) / float64(deltaTot) * 100.0
				if pct > 100.0 {
					pct = 100.0
				}
				m.CPUPercent = pct
			}
		}

		newMetrics[name] = m
		newPrevSamples[name] = prevCPUSample{procTicks: procTicks, totalTicks: totalTicks}
	}

	c.mu.Lock()
	c.metrics = newMetrics
	c.prevSamples = newPrevSamples
	c.mu.Unlock()
}

// findPIDByComm scans /proc for a process whose comm file matches name exactly.
func findPIDByComm(name string) (int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, err
	}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		commBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(commBytes)) == name {
			return pid, nil
		}
	}
	return 0, fmt.Errorf("process %q not found in /proc", name)
}

// readProcCPUTicks returns utime+stime (in clock ticks) from /proc/<pid>/stat.
func readProcCPUTicks(pid int) (uint64, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, err
	}
	// comm can contain spaces and parentheses; find the last ')' and parse after it.
	s := string(data)
	rp := strings.LastIndex(s, ")")
	if rp < 0 {
		return 0, fmt.Errorf("malformed /proc/%d/stat", pid)
	}
	// After ')': state ppid pgrp session tty_nr tty_pgrp flags
	//            min_flt cmin_flt maj_flt cmaj_flt utime stime ...
	fields := strings.Fields(s[rp+1:])
	if len(fields) < 13 {
		return 0, fmt.Errorf("not enough fields in /proc/%d/stat", pid)
	}
	utime, err := strconv.ParseUint(fields[11], 10, 64)
	if err != nil {
		return 0, err
	}
	stime, err := strconv.ParseUint(fields[12], 10, 64)
	if err != nil {
		return 0, err
	}
	return utime + stime, nil
}

// readProcMemRSSKB returns VmRSS in kB from /proc/<pid>/status.
func readProcMemRSSKB(pid int) (uint64, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return strconv.ParseUint(fields[1], 10, 64)
			}
		}
	}
	return 0, fmt.Errorf("VmRSS not found in /proc/%d/status", pid)
}

// readTotalCPUTicks returns the sum of all CPU time fields from the first line
// of /proc/stat ("cpu  user nice system idle iowait irq softirq steal ...").
// This is the aggregate across all logical cores.
func readTotalCPUTicks() (uint64, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}
	firstLine := strings.SplitN(string(data), "\n", 2)[0]
	fields := strings.Fields(firstLine)
	if len(fields) < 2 || fields[0] != "cpu" {
		return 0, fmt.Errorf("unexpected /proc/stat format")
	}
	var total uint64
	for _, f := range fields[1:] {
		v, err := strconv.ParseUint(f, 10, 64)
		if err != nil {
			continue
		}
		total += v
	}
	return total, nil
}
