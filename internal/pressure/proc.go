//go:build linux

package pressure

import (
	"bufio"
	"bytes"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

var platformPolicy = linuxPolicy

// collect leaves a metric unobserved on any read or parse failure: the wire names
// it in missing, and older kernels lack the PSI files entirely.
func (collector *collector) collect() Signals {
	signals := Signals{}
	if contents, err := os.ReadFile("/proc/stat"); err == nil {
		if counters, ok := parseCPUCounters(contents); ok {
			collector.observeCPU(signals, counters)
		}
	}
	if contents, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(contents))
		if len(fields) > 0 && runtime.NumCPU() > 0 {
			if load, parseErr := strconv.ParseFloat(fields[0], 64); parseErr == nil && load >= 0 && !math.IsInf(load, 0) {
				signals.record(normalizedLoad, load/float64(runtime.NumCPU()))
			}
		}
	}
	if contents, err := os.ReadFile("/proc/meminfo"); err == nil {
		memory := parseMemory(contents)
		total, totalKnown := memory["MemTotal"]
		available, availableKnown := memory["MemAvailable"]
		if totalKnown && availableKnown && total > 0 && available <= total {
			signals.record(memoryAvailablePercent, available*100/total)
		}
		swapTotal, swapTotalKnown := memory["SwapTotal"]
		swapFree, swapFreeKnown := memory["SwapFree"]
		if swapTotalKnown && swapFreeKnown && swapFree <= swapTotal {
			if swapTotal == 0 {
				signals.record(swapUsedPercent, 0)
			} else {
				signals.record(swapUsedPercent, (swapTotal-swapFree)*100/swapTotal)
			}
		}
	}
	var filesystem syscall.Statfs_t
	if err := syscall.Statfs("/", &filesystem); err == nil && filesystem.Blocks > 0 && filesystem.Bavail <= filesystem.Blocks {
		signals.record(diskAvailablePercent, float64(filesystem.Bavail)*100/float64(filesystem.Blocks))
	}
	if contents, err := os.ReadFile("/proc/pressure/cpu"); err == nil {
		if psi, known := parsePSI(contents, "some"); known {
			signals.record(cpuPSISomeAvg60Percent, psi)
		}
	}
	if contents, err := os.ReadFile("/proc/pressure/memory"); err == nil {
		if psi, known := parsePSI(contents, "full"); known {
			signals.record(memoryPSIFullAvg60Percent, psi)
		}
	}
	if contents, err := os.ReadFile("/proc/pressure/io"); err == nil {
		if psi, known := parsePSI(contents, "full"); known {
			signals.record(ioPSIFullAvg60Percent, psi)
		}
	}
	return signals
}

func parseCPUCounters(contents []byte) (cpuCounters, bool) {
	line, _, _ := bytes.Cut(contents, []byte{'\n'})
	fields := strings.Fields(string(line))
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuCounters{}, false
	}
	limit := len(fields)
	if limit > 9 {
		limit = 9
	}
	values := make([]uint64, limit-1)
	for index := 1; index < limit; index++ {
		value, err := strconv.ParseUint(fields[index], 10, 64)
		if err != nil {
			return cpuCounters{}, false
		}
		values[index-1] = value
	}
	var total uint64
	for _, value := range values {
		total += value
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	return cpuCounters{total: total, idle: idle}, total > 0
}

// parseMemory returns the parsed MemTotal, MemAvailable, SwapTotal and SwapFree kB values;
// an absent or malformed line has no entry.
func parseMemory(contents []byte) map[string]float64 {
	values := map[string]float64{}
	scanner := bufio.NewScanner(bytes.NewReader(contents))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 3 || fields[2] != "kB" {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		switch key {
		case "MemTotal", "MemAvailable", "SwapTotal", "SwapFree":
			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err == nil {
				values[key] = float64(value)
			}
		}
	}
	return values
}

func parsePSI(contents []byte, kind string) (float64, bool) {
	scanner := bufio.NewScanner(bytes.NewReader(contents))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 || fields[0] != kind {
			continue
		}
		for _, field := range fields[1:] {
			value, found := strings.CutPrefix(field, "avg60=")
			if !found {
				continue
			}
			parsed, err := strconv.ParseFloat(value, 64)
			if err == nil && parsed >= 0 && parsed <= 100 && !math.IsInf(parsed, 0) {
				return parsed, true
			}
			return 0, false
		}
	}
	return 0, false
}
