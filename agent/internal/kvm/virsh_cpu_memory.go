package kvm

import (
	"encoding/xml"
	"strconv"
	"strings"
	"time"
)

type cpuSample struct {
	timeNS    int64
	sampledAt time.Time
}

type cpuUsageSample struct {
	usage     int
	available bool
}

type ioSample struct {
	diskReadBytes  int64
	diskWriteBytes int64
	networkRxBytes int64
	networkTxBytes int64
	sampledAt      time.Time
}

type ioRateSample struct {
	diskReadBytesPerSec  int64
	diskWriteBytesPerSec int64
	networkRxBytesPerSec int64
	networkTxBytesPerSec int64
	available            bool
}

func (p *VirshProvider) sampleCPUUsages(names []string) map[string]cpuUsageSample {
	first := p.cpuTimeSamples(names)
	second := map[string]cpuSample{}
	if len(first) > 0 {
		time.Sleep(time.Second)
		second = p.cpuTimeSamples(names)
	}
	return p.cpuUsageRates(first, second)
}

func (p *VirshProvider) sampleIORates(names []string) map[string]ioRateSample {
	first := p.ioStatSamples(names)
	second := map[string]ioSample{}
	if len(first) > 0 {
		time.Sleep(time.Second)
		second = p.ioStatSamples(names)
	}
	return ioRates(first, second)
}

func (p *VirshProvider) sampleRuntimeRates(names []string) (map[string]cpuUsageSample, map[string]ioRateSample) {
	if len(names) == 0 {
		return map[string]cpuUsageSample{}, map[string]ioRateSample{}
	}
	firstCPU := p.cpuTimeSamples(names)
	firstIO := p.ioStatSamples(names)
	if len(firstCPU) == 0 && len(firstIO) == 0 {
		return map[string]cpuUsageSample{}, map[string]ioRateSample{}
	}
	time.Sleep(time.Second)
	secondCPU := map[string]cpuSample{}
	if len(firstCPU) > 0 {
		secondCPU = p.cpuTimeSamples(names)
	}
	secondIO := map[string]ioSample{}
	if len(firstIO) > 0 {
		secondIO = p.ioStatSamples(names)
	}
	return p.cpuUsageRates(firstCPU, secondCPU), ioRates(firstIO, secondIO)
}

func (p *VirshProvider) cpuUsageRates(first map[string]cpuSample, second map[string]cpuSample) map[string]cpuUsageSample {
	result := make(map[string]cpuUsageSample, len(first))
	for name, start := range first {
		end, ok := second[name]
		if !ok || end.timeNS < start.timeNS || !start.sampledAt.Before(end.sampledAt) {
			continue
		}
		elapsedNS := end.sampledAt.Sub(start.sampledAt).Nanoseconds()
		if elapsedNS <= 0 {
			continue
		}
		cpuCores := p.currentVCPU(name)
		if cpuCores <= 0 {
			cpuCores = 1
		}
		usage := int((end.timeNS - start.timeNS) * 100 / (elapsedNS * int64(cpuCores)))
		result[name] = cpuUsageSample{usage: clampPercent(usage), available: true}
	}
	return result
}

func ioRates(first map[string]ioSample, second map[string]ioSample) map[string]ioRateSample {
	result := make(map[string]ioRateSample, len(first))
	for name, start := range first {
		end, ok := second[name]
		if !ok || !start.sampledAt.Before(end.sampledAt) {
			continue
		}
		elapsed := end.sampledAt.Sub(start.sampledAt).Seconds()
		if elapsed <= 0 {
			continue
		}
		result[name] = ioRateSample{
			diskReadBytesPerSec:  deltaPerSecond(start.diskReadBytes, end.diskReadBytes, elapsed),
			diskWriteBytesPerSec: deltaPerSecond(start.diskWriteBytes, end.diskWriteBytes, elapsed),
			networkRxBytesPerSec: deltaPerSecond(start.networkRxBytes, end.networkRxBytes, elapsed),
			networkTxBytesPerSec: deltaPerSecond(start.networkTxBytes, end.networkTxBytes, elapsed),
			available:            true,
		}
	}
	return result
}

func (p *VirshProvider) ioStatSamples(names []string) map[string]ioSample {
	out, err := p.output("virsh", append([]string{"--connect", p.libvirtURI, "domstats", "--block", "--interface"}, names...)...)
	if err != nil {
		return map[string]ioSample{}
	}
	return parseIOStatSamples(out, time.Now())
}

func (p *VirshProvider) cpuTimeSamples(names []string) map[string]cpuSample {
	out, err := p.output("virsh", append([]string{"--connect", p.libvirtURI, "domstats", "--cpu-total"}, names...)...)
	if err != nil {
		return map[string]cpuSample{}
	}
	return parseCPUTimeSamples(out, time.Now())
}

func parseCPUTimeSamples(out string, sampledAt time.Time) map[string]cpuSample {
	items := map[string]cpuSample{}
	current := ""
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Domain:") {
			current = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "Domain:")), "'\"")
			continue
		}
		if current == "" || !strings.HasPrefix(line, "cpu.time=") {
			continue
		}
		value, _ := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, "cpu.time=")), 10, 64)
		if value > 0 {
			items[current] = cpuSample{timeNS: value, sampledAt: sampledAt}
		}
	}
	return items
}

func parseIOStatSamples(out string, sampledAt time.Time) map[string]ioSample {
	items := map[string]ioSample{}
	current := ""
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Domain:") {
			current = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "Domain:")), "'\"")
			items[current] = ioSample{sampledAt: sampledAt}
			continue
		}
		if current == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		value, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if value < 0 {
			continue
		}
		sample := items[current]
		switch {
		case strings.HasSuffix(parts[0], ".rd.bytes"):
			sample.diskReadBytes += value
		case strings.HasSuffix(parts[0], ".wr.bytes"):
			sample.diskWriteBytes += value
		case strings.HasSuffix(parts[0], ".rx.bytes"):
			sample.networkRxBytes += value
		case strings.HasSuffix(parts[0], ".tx.bytes"):
			sample.networkTxBytes += value
		}
		items[current] = sample
	}
	return items
}

func deltaPerSecond(start, end int64, elapsed float64) int64 {
	if end < start || elapsed <= 0 {
		return 0
	}
	return int64(float64(end-start) / elapsed)
}

func (p *VirshProvider) currentVCPU(name string) int {
	out, err := p.output("virsh", "--connect", p.libvirtURI, "dumpxml", name)
	if err != nil {
		return 0
	}
	var doc domainXML
	if err := xml.Unmarshal([]byte(out), &doc); err != nil {
		return 0
	}
	if doc.VCPU.Current > 0 {
		return doc.VCPU.Current
	}
	return doc.VCPU.Value
}

func (p *VirshProvider) vmMemoryUsage(name string, memoryBytes int64, status string) (int, bool) {
	if status != "running" || memoryBytes <= 0 {
		return 0, status != "running"
	}
	if out, err := p.output("virsh", "--connect", p.libvirtURI, "dommemstat", name); err == nil {
		actual, actualOK := parseMemstatKiB(out, "actual")
		usable, usableOK := parseMemstatKiB(out, "usable")
		if actualOK && actual > 0 && usableOK && usable >= 0 && usable <= actual {
			return clampPercent(int((actual - usable) * 100 / actual)), true
		}
		available, availableOK := parseMemstatKiB(out, "available")
		if actualOK && actual > 0 && availableOK && available >= 0 && available <= actual {
			return clampPercent(int((actual - available) * 100 / actual)), true
		}
	}
	return 0, false
}

func parseDomstatsInt64(out string, key string) int64 {
	needle := key + "="
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, needle) {
			value, _ := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, needle)), 10, 64)
			return value
		}
	}
	return 0
}

func parseMemstatKiB(out string, key string) (int64, bool) {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == key {
			value, _ := strconv.ParseInt(fields[1], 10, 64)
			return value, true
		}
	}
	return 0, false
}
