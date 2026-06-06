package router

import (
	"testing"

	"kvm-manager/backend/pkg/agent"
)

func TestValidateCreateVMRequestRejectsUnsupportedISOBus(t *testing.T) {
	body := validCreateVMRequest()
	body.ISOBus = "virtio"
	if err := validateCreateVMRequest(body); err == nil {
		t.Fatal("expected unsupported iso bus error")
	}
}

func TestValidateCreateVMRequestAllowsDefaultISOBus(t *testing.T) {
	body := validCreateVMRequest()
	if err := validateCreateVMRequest(body); err != nil {
		t.Fatalf("validate create vm request failed: %v", err)
	}
}

func TestValidateCreateVMRequestAllowsXMLWithoutHardwareFields(t *testing.T) {
	body := createVMRequest{
		AgentID: "agent-01",
		XML:     "<domain type='kvm'><name>demo</name></domain>",
	}
	if err := validateCreateVMRequest(body); err != nil {
		t.Fatalf("validate xml create vm request failed: %v", err)
	}
}

func TestValidateCreateVMRequestAllowsTemplateMode(t *testing.T) {
	body := validCreateVMRequest()
	body.CreateMode = "template"
	body.Disks = nil
	body.Template = createVMTemplate{
		SourcePool: "default",
		SourceName: "centos-template.qcow2",
		TargetPool: "default",
		TargetName: "demo-vda.qcow2",
		Bus:        "virtio",
		Format:     "qcow2",
	}
	if err := validateCreateVMRequest(body); err != nil {
		t.Fatalf("validate template create vm request failed: %v", err)
	}
}

func TestValidateCreateVMRequestRejectsTemplateTargetExtension(t *testing.T) {
	body := validCreateVMRequest()
	body.CreateMode = "template"
	body.Disks = nil
	body.Template = createVMTemplate{
		SourcePool: "default",
		SourceName: "centos-template.qcow2",
		TargetPool: "default",
		TargetName: "demo-vda.img",
		Bus:        "virtio",
		Format:     "qcow2",
	}
	if err := validateCreateVMRequest(body); err == nil {
		t.Fatal("expected template target extension error")
	}
}

func TestValidateCreateVMRequestTemplateModeStillRequiresNetwork(t *testing.T) {
	body := validCreateVMRequest()
	body.CreateMode = "template"
	body.Disks = nil
	body.NetworkSource = ""
	body.Template = createVMTemplate{
		SourcePool: "default",
		SourceName: "centos-template.qcow2",
		TargetPool: "default",
		TargetName: "demo-vda.qcow2",
		Bus:        "virtio",
		Format:     "qcow2",
	}
	if err := validateCreateVMRequest(body); err == nil {
		t.Fatal("expected network required error")
	}
}

func TestValidateCreateVMRequestRejectsDuplicateDiskNames(t *testing.T) {
	body := validCreateVMRequest()
	body.Disks = append(body.Disks, createVMDiskRequest{
		Name:       "demo-vda.qcow2",
		Pool:       "default",
		Format:     "qcow2",
		Bus:        "virtio",
		CapacityGB: 20,
	})
	if err := validateCreateVMRequest(body); err == nil {
		t.Fatal("expected duplicate disk name error")
	}
}

func TestValidateCreateVMRequestRejectsDataDiskSettingsMismatch(t *testing.T) {
	body := validCreateVMRequest()
	body.Disks = append(body.Disks, createVMDiskRequest{
		Name:       "demo-vdb.qcow2",
		Pool:       "secondary",
		Format:     "qcow2",
		Bus:        "virtio",
		CapacityGB: 20,
	})
	if err := validateCreateVMRequest(body); err == nil {
		t.Fatal("expected data disk settings mismatch error")
	}
}

func TestValidateCreateVMRequestRejectsDiskPathSeparator(t *testing.T) {
	body := validCreateVMRequest()
	body.Disks[0].Name = "nested/demo-vda.qcow2"
	if err := validateCreateVMRequest(body); err == nil {
		t.Fatal("expected disk path separator error")
	}
}

func TestValidateHostResourceLimitsRejectsOverflow(t *testing.T) {
	if status, code, message := validateHostResourceLimits(4, 8*1024*1024*1024, 8, 4096); status == 0 || code == "" || message == "" {
		t.Fatal("expected cpu limit error")
	}
	if status, code, message := validateHostResourceLimits(8, 4*1024*1024*1024, 4, 8192); status == 0 || code == "" || message == "" {
		t.Fatal("expected memory limit error")
	}
}

func TestFriendlyVMCreateTaskMessageTemplateWriteLock(t *testing.T) {
	request := agent.VMCreateRequest{
		CreateMode: "template",
		Template: agent.VMCreateTemplate{
			SourceName: "ct7-template.qcow2",
		},
	}
	input := `qemu-img convert failed: exit status 1: qemu-img: Failed to get shared "write" lock Is another process using the image [/kvm/vm/ct7-template.qcow2]?`
	got := friendlyVMCreateTaskMessage(request, input)
	want := "模板文件 ct7-template.qcow2 正在被运行中的虚拟机使用，无法克隆。请先关闭使用该模板文件的虚拟机后再创建"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestFriendlyVMCreateTaskMessageBlankKeepsOriginal(t *testing.T) {
	request := agent.VMCreateRequest{CreateMode: "blank"}
	input := `qemu-img convert failed: exit status 1: qemu-img: Failed to get shared "write" lock`
	if got := friendlyVMCreateTaskMessage(request, input); got != input {
		t.Fatalf("expected original message, got %q", got)
	}
}

func validCreateVMRequest() createVMRequest {
	return createVMRequest{
		AgentID:         "agent-01",
		Name:            "demo",
		CurrentCPU:      2,
		MaximumCPU:      2,
		CurrentMemoryMB: 4096,
		MaximumMemoryMB: 4096,
		Disks: []createVMDiskRequest{{
			Name:       "demo-vda.qcow2",
			Pool:       "default",
			Format:     "qcow2",
			Bus:        "virtio",
			CapacityGB: 40,
		}},
		NetworkSource: "default",
	}
}
