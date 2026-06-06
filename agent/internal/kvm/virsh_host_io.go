package kvm

import (
	"strconv"
	"strings"
	"time"
)

type hostIOSample struct {
	diskReadBytes  int64
	diskWriteBytes int64
	networkRxBytes int64
	networkTxBytes int64
	sampledAt      time.Time
}

func (p *VirshProvider) hostIORates() ioRateSample {
	start := p.hostIOSample()
	if start.sampledAt.IsZero() {
		return ioRateSample{}
	}
	time.Sleep(time.Second)
	end := p.hostIOSample()
	if end.sampledAt.IsZero() || !start.sampledAt.Before(end.sampledAt) {
		return ioRateSample{}
	}
	elapsed := end.sampledAt.Sub(start.sampledAt).Seconds()
	if elapsed <= 0 {
		return ioRateSample{}
	}
	return ioRateSample{
		diskReadBytesPerSec:  deltaPerSecond(start.diskReadBytes, end.diskReadBytes, elapsed),
		diskWriteBytesPerSec: deltaPerSecond(start.diskWriteBytes, end.diskWriteBytes, elapsed),
		networkRxBytesPerSec: deltaPerSecond(start.networkRxBytes, end.networkRxBytes, elapsed),
		networkTxBytesPerSec: deltaPerSecond(start.networkTxBytes, end.networkTxBytes, elapsed),
		available:            true,
	}
}

func (p *VirshProvider) hostIOSample() hostIOSample {
	return hostIOSample{
		diskReadBytes:  p.hostDiskBytes("read"),
		diskWriteBytes: p.hostDiskBytes("write"),
		networkRxBytes: p.hostNetworkBytes("rx"),
		networkTxBytes: p.hostNetworkBytes("tx"),
		sampledAt:      time.Now(),
	}
}

func (p *VirshProvider) hostDiskBytes(direction string) int64 {
	column := "$6"
	if direction == "write" {
		column = "$10"
	}
	cmd := "awk -F= '$1==\"DEVTYPE\" && $2==\"disk\" {name=FILENAME; sub(\".*/\", \"\", name); sub(\"/uevent$\", \"\", name); print name}' /sys/block/*/uevent 2>/dev/null | " +
		"awk 'NR==FNR {dev[$1]=1; next} ($3 in dev) {sum += " + column + " * 512} END {print sum+0}' - /proc/diskstats"
	out, err := p.output("sh", "-c", cmd)
	if err != nil {
		return 0
	}
	value, _ := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	return value
}

func (p *VirshProvider) hostNetworkBytes(direction string) int64 {
	field := "$2"
	if direction == "tx" {
		field = "$10"
	}
	cmd := "awk 'NR>2 {iface=$1; sub(/:$/, \"\", iface); if (iface != \"lo\") sum += " + field + "} END {print sum+0}' /proc/net/dev"
	out, err := p.output("sh", "-c", cmd)
	if err != nil {
		return 0
	}
	value, _ := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	return value
}
