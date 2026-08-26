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

type metric struct {
	value float64
	known bool
}

func knownMetric(value float64) metric { return metric{value: value, known: true} }

type rawSample struct {
	cpuPercent             metric
	loadNormalized         metric
	memoryAvailablePercent metric
	swapUsedPercent        metric
	diskAvailablePercent   metric
	cpuPSISomeAvg60        metric
	memoryPSIFullAvg60     metric
	ioPSIFullAvg60         metric
}

type cpuCounters struct {
	total uint64
	idle  uint64
}

type collector struct {
	previousCPU    cpuCounters
	hasPreviousCPU bool
}

func newCollector() collector { return collector{} }

func (collector *collector) collect() rawSample {
	sample := rawSample{}
	if contents, err := os.ReadFile("/proc/stat"); err == nil {
		if counters, ok := parseCPUCounters(contents); ok {
			if collector.hasPreviousCPU {
				sample.cpuPercent = cpuUsage(collector.previousCPU, counters)
			}
			collector.previousCPU = counters
			collector.hasPreviousCPU = true
		}
	}
	if contents, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(contents))
		if len(fields) > 0 && runtime.NumCPU() > 0 {
			if load, parseErr := strconv.ParseFloat(fields[0], 64); parseErr == nil && load >= 0 && !math.IsInf(load, 0) {
				sample.loadNormalized = knownMetric(load / float64(runtime.NumCPU()))
			}
		}
	}
	if contents, err := os.ReadFile("/proc/meminfo"); err == nil {
		total, available, swapTotal, swapFree := parseMemory(contents)
		if total.known && available.known && total.value > 0 && available.value <= total.value {
			sample.memoryAvailablePercent = knownMetric(available.value * 100 / total.value)
		}
		if swapTotal.known && swapFree.known && swapTotal.value >= 0 && swapFree.value <= swapTotal.value {
			if swapTotal.value == 0 {
				sample.swapUsedPercent = knownMetric(0)
			} else {
				sample.swapUsedPercent = knownMetric((swapTotal.value - swapFree.value) * 100 / swapTotal.value)
			}
		}
	}
	var filesystem syscall.Statfs_t
	if err := syscall.Statfs("/", &filesystem); err == nil && filesystem.Blocks > 0 && filesystem.Bavail <= filesystem.Blocks {
		sample.diskAvailablePercent = knownMetric(float64(filesystem.Bavail) * 100 / float64(filesystem.Blocks))
	}
	if contents, err := os.ReadFile("/proc/pressure/cpu"); err == nil {
		sample.cpuPSISomeAvg60 = parsePSI(contents, "some")
	}
	if contents, err := os.ReadFile("/proc/pressure/memory"); err == nil {
		sample.memoryPSIFullAvg60 = parsePSI(contents, "full")
	}
	if contents, err := os.ReadFile("/proc/pressure/io"); err == nil {
		sample.ioPSIFullAvg60 = parsePSI(contents, "full")
	}
	return sample
}

func cpuUsage(previous, current cpuCounters) metric {
	if current.total <= previous.total || current.idle < previous.idle {
		return metric{}
	}
	total := current.total - previous.total
	idle := current.idle - previous.idle
	if idle > total {
		return metric{}
	}
	return knownMetric(float64(total-idle) * 100 / float64(total))
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

func parseMemory(contents []byte) (metric, metric, metric, metric) {
	values := map[string]metric{}
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
				values[key] = knownMetric(float64(value))
			}
		}
	}
	return values["MemTotal"], values["MemAvailable"], values["SwapTotal"], values["SwapFree"]
}

func parsePSI(contents []byte, kind string) metric {
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
				return knownMetric(parsed)
			}
			return metric{}
		}
	}
	return metric{}
}
