package kvm

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type VirshProvider struct {
	libvirtURI string
	timeout    time.Duration
	logger     *slog.Logger
}

func NewVirshProvider(libvirtURI string, timeout time.Duration) *VirshProvider {
	return &VirshProvider{libvirtURI: libvirtURI, timeout: timeout}
}

func (p *VirshProvider) WithLogger(logger *slog.Logger) *VirshProvider {
	p.logger = logger
	return p
}

func (p *VirshProvider) ListVMs() ([]VirtualMachine, error) {
	return p.listVMs(false)
}

func (p *VirshProvider) ListVMsFast() ([]VirtualMachine, error) {
	return p.listVMs(true)
}

func (p *VirshProvider) VM(vmName string) (VirtualMachine, error) {
	vmName = strings.TrimSpace(vmName)
	if vmName == "" {
		return VirtualMachine{}, fmt.Errorf("vm name is required")
	}
	cpuUsages, ioRates := p.sampleRuntimeRates([]string{vmName})
	return p.describeVM(vmName, cpuUsages[vmName], ioRates[vmName], false)
}

func (p *VirshProvider) listVMs(fast bool) ([]VirtualMachine, error) {
	out, err := p.output("virsh", "--connect", p.libvirtURI, "list", "--all", "--name")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0)
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	cpuUsages, ioRates := p.sampleRuntimeRates(names)
	vms := make([]VirtualMachine, 0, len(names))
	if fast {
		for _, name := range names {
			vm, err := p.describeVM(name, cpuUsages[name], ioRates[name], fast)
			if err != nil {
				vm = VirtualMachine{Name: name, Status: "unknown"}
			}
			vms = append(vms, vm)
		}
		return vms, nil
	}
	vms = p.describeVMs(names, cpuUsages, ioRates)
	return vms, nil
}

func (p *VirshProvider) describeVMs(names []string, cpuUsages map[string]cpuUsageSample, ioRates map[string]ioRateSample) []VirtualMachine {
	vms := make([]VirtualMachine, len(names))
	filesystemsByVM := p.guestFilesystemUsagesByVM(names)
	workers := 4
	if workers > len(names) {
		workers = len(names)
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				name := names[index]
				vm, err := p.describeVMWithFilesystems(name, cpuUsages[name], ioRates[name], false, filesystemsByVM[name])
				if err != nil {
					vm = VirtualMachine{Name: name, Status: "unknown"}
				}
				vms[index] = vm
			}
		}()
	}
	for index := range names {
		jobs <- index
	}
	close(jobs)
	wg.Wait()
	return vms
}

func (p *VirshProvider) ListSnapshots(vmName string) ([]Snapshot, error) {
	out, err := p.output("virsh", "--connect", p.libvirtURI, "snapshot-list", vmName)
	if err != nil {
		return nil, err
	}
	return parseSnapshotList(out), nil
}

func parseSnapshotList(out string) []Snapshot {
	snapshots := make([]Snapshot, 0)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Name ") || strings.HasPrefix(line, "----") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}
		snapshot := Snapshot{Name: fields[0], Status: "ready"}
		if len(fields) >= 4 {
			snapshot.CreatedAt = fields[1] + " " + fields[2] + " " + fields[3]
		}
		if len(fields) >= 5 {
			snapshot.Status = strings.Join(fields[4:], " ")
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

func (p *VirshProvider) describeVM(name string, cpuSample cpuUsageSample, ioSample ioRateSample, fast bool) (VirtualMachine, error) {
	return p.describeVMWithFilesystems(name, cpuSample, ioSample, fast, nil)
}

func (p *VirshProvider) describeVMWithFilesystems(name string, cpuSample cpuUsageSample, ioSample ioRateSample, fast bool, filesystems []guestFilesystemUsage) (VirtualMachine, error) {
	stateOut, _ := p.output("virsh", "--connect", p.libvirtURI, "domstate", name)
	_, doc, err := p.persistentDomainXML(name)
	if err != nil {
		return VirtualMachine{}, err
	}
	status := normalizeState(stateOut)
	disks, diskTotal, diskUsed, diskAvailable := p.diskDetails(name, doc, fast, filesystems)
	diskUsage := 0
	if status == "running" && diskAvailable && diskTotal > 0 {
		diskUsage = clampPercent(int(diskUsed * 100 / diskTotal))
	}
	diskUsageAvailable := diskAvailable || status != "running"
	cpuCores := doc.VCPU.Current
	if cpuCores <= 0 {
		cpuCores = doc.VCPU.Value
	}
	cpuUsage, cpuAvailable := 0, true
	if status == "running" {
		cpuUsage = cpuSample.usage
		cpuAvailable = cpuSample.available
	}
	memoryDoc := doc
	if status == "running" {
		if _, inactiveDoc, err := p.inactiveDomainXML(name); err == nil {
			memoryDoc = inactiveDoc
		}
	}
	currentMemory, _ := configMemoryKiB(memoryDoc)
	memoryUsage, memoryAvailable := p.vmMemoryUsage(name, kibToBytes(currentMemory), status)
	osType := ""
	primaryIP := ""
	if !fast {
		osType = p.detectOSType(name, doc)
		primaryIP = p.primaryIP(name)
	}
	return VirtualMachine{
		Name:                 doc.Name,
		UUID:                 doc.UUID,
		Description:          strings.TrimSpace(doc.Description),
		OSType:               osType,
		Status:               status,
		CPUCores:             cpuCores,
		MemoryBytes:          kibToBytes(currentMemory),
		DiskBytes:            diskTotal,
		DiskUsedBytes:        diskUsed,
		Disks:                disks,
		PrimaryIP:            primaryIP,
		CPUUsage:             cpuUsage,
		CPUUsageAvailable:    cpuAvailable,
		MemoryUsage:          memoryUsage,
		MemoryUsageAvailable: memoryAvailable,
		DiskUsage:            diskUsage,
		DiskUsageAvailable:   diskUsageAvailable,
		DiskReadBytesPerSec:  ioSample.diskReadBytesPerSec,
		DiskWriteBytesPerSec: ioSample.diskWriteBytesPerSec,
		NetworkRxBytesPerSec: ioSample.networkRxBytesPerSec,
		NetworkTxBytesPerSec: ioSample.networkTxBytesPerSec,
		UptimeSeconds:        p.vmUptimeSeconds(name, doc.UUID, status),
	}, nil
}

func (p *VirshProvider) output(name string, args ...string) (string, error) {
	return p.outputWithTimeout(p.timeout, name, args...)
}

func (p *VirshProvider) storageOutput(name string, args ...string) (string, error) {
	return p.outputWithoutTimeout(name, args...)
}

func (p *VirshProvider) outputWithTimeout(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	applyCommandEnvironment(cmd, name)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return "", fmt.Errorf("%s timed out", name)
	}
	if err != nil {
		return "", fmt.Errorf("%s %s failed: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (p *VirshProvider) outputWithoutTimeout(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	applyCommandEnvironment(cmd, name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s failed: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func applyCommandEnvironment(cmd *exec.Cmd, name string) {
	if !isLibguestfsCommand(name) || hasEnvVar(os.Environ(), "LIBGUESTFS_BACKEND") {
		return
	}
	cmd.Env = append(os.Environ(), "LIBGUESTFS_BACKEND=direct")
}

func isLibguestfsCommand(name string) bool {
	switch strings.TrimSpace(name) {
	case "virt-df", "virt-filesystems":
		return true
	default:
		return false
	}
}

func hasEnvVar(env []string, key string) bool {
	prefix := key + "="
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			return true
		}
	}
	return false
}
