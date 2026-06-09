package router

import (
	"errors"
	"strings"
	"testing"
)

func TestOperationErrorMessageForMissingStorageFile(t *testing.T) {
	err := errors.New("virsh --connect qemu:///system start ct7-template failed: exit status 1: error: Failed to start domain 'ct7-template' error: Cannot access storage file '/kvm/iso/CentOS-7.9-x86_64-DVD-2009.iso': No such file or directory")

	message := operationErrorMessage("虚拟机操作失败", err)

	if !strings.Contains(message, "虚拟机挂载的镜像或光驱文件不存在") {
		t.Fatalf("unexpected message: %s", message)
	}
	if strings.Contains(message, "虚拟机操作失败") {
		t.Fatalf("message should not include operation summary: %s", message)
	}
}

func TestOperationErrorMessageForMigrationMissingStorageFile(t *testing.T) {
	err := errors.New("virsh --connect qemu:///system migrate --live --persistent --auto-converge vm1 qemu+ssh://192.168.51.47/system failed: exit status 1: error: Cannot access storage file '/data/kvm/ct7.qcow2' (as uid:0, gid:0): No such file or directory")

	message := operationErrorMessage("迁移虚拟机失败", err)

	expected := "目标宿主机无法访问源磁盘路径 /data/kvm/ct7.qcow2，请确认目标宿主机存在该路径所在目录或存储池后重试"
	if message != expected {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestOperationErrorMessageForMigrationGuestMemoryAllocation(t *testing.T) {
	err := errors.New("virsh --connect qemu:///system migrate --live --persistent --auto-converge vm1 qemu+ssh://192.168.51.47/system failed: exit status 1: error: internal error: qemu unexpectedly closed the monitor: Cannot set up guest memory 'pc.ram': Cannot allocate memory")

	message := operationErrorMessage("迁移虚拟机失败", err)

	expected := "目标宿主机可用内存不足，无法为迁移虚拟机分配内存，请释放目标宿主机内存或降低虚拟机内存后重试"
	if message != expected {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestOperationErrorMessageForDuplicateStart(t *testing.T) {
	err := errors.New("virsh --connect qemu:///system start ct7-template failed: exit status 1: error: Failed to start domain 'ct7-template' error: Requested operation is not valid: domain is already running")

	message := operationErrorMessage("虚拟机操作失败", err)

	if message != "虚拟机当前已运行" {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestOperationErrorMessageForDuplicateShutdown(t *testing.T) {
	err := errors.New("virsh --connect qemu:///system shutdown ct7-template failed: exit status 1: error: Failed to shutdown domain 'ct7-template' error: Requested operation is not valid: domain is not running")

	message := operationErrorMessage("虚拟机操作失败", err)

	if message != "虚拟机当前已关机" {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestOperationErrorMessageForDeleteRunningVM(t *testing.T) {
	err := errors.New("vm 192.168.12.179 must be stopped before delete")

	message := operationErrorMessage("虚拟机操作失败", err)

	if message != "请先关闭虚拟机后再删除" {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestOperationErrorMessageForDeleteVMWithSnapshots(t *testing.T) {
	err := errors.New("virsh --connect qemu:///system undefine test failed: exit status 1: error: Failed to undefine domain test error: Requested operation is not valid: cannot delete inactive domain with 2 snapshots")

	message := operationErrorMessage("删除虚拟机失败", err)

	expected := "删除虚拟机失败：仍存在 2 个快照，请先删除快照后重试"
	if message != expected {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestOperationErrorMessageForStoragePoolUnmountBusy(t *testing.T) {
	err := errors.New("virsh --connect qemu:///system pool-destroy nfs failed: exit status 1: error: Failed to destroy pool nfs\nerror: internal error: Child process (/usr/bin/umount /data/nfs) unexpected exit status 16: umount.nfs4: /data/nfs: device is busy")

	message := operationErrorMessage("修改存储池状态失败", err)

	expected := "存储池挂载目录正在被使用中，无法停止"
	if message != expected {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestOperationErrorMessageForStoragePoolSourceConflict(t *testing.T) {
	err := errors.New("virsh --connect qemu:///system pool-define-as nfs dir --target /data/kvm failed: exit status 1: error: Failed to define pool nfs\nerror: operation failed: Storage source conflict with pool: 'Storage'")

	message := operationErrorMessage("创建存储池失败", err)

	expected := "存储池路径已被存储池 Storage 使用，请更换路径或先处理该存储池"
	if message != expected {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestVMDeleteHasSnapshotsMessageFallback(t *testing.T) {
	message := vmDeleteHasSnapshotsMessage("cannot delete inactive domain with snapshots")

	expected := "删除虚拟机失败：仍存在快照，请先删除快照后重试"
	if message != expected {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestOperationErrorMessageForDiskResizeWithInternalSnapshots(t *testing.T) {
	err := errors.New("qemu-img resize /kvm/iso/demo.qcow2 23622320128 failed: exit status 1: qemu-img: Can't resize an image which has snapshots qemu-img: This image does not support resize")

	message := operationErrorMessage("修改虚拟机设备失败", err)

	expected := "磁盘镜像包含内部快照，无法直接扩容。请先删除该虚拟机的内部快照，或重新创建无内部快照的磁盘后再扩容"
	if message != expected {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestOperationErrorMessageForSnapshotRevertMissingDisk(t *testing.T) {
	err := errors.New("virsh --connect qemu:///system snapshot-revert test snap-test-202606040933 failed: exit status 1: error: internal error: Child process (/usr/bin/qemu-img snapshot -a snap-test-202606040933 /kvm/iso/test-vdb.qcow2) unexpected exit status 1: qemu-img: Could not open '/kvm/iso/test-vdb.qcow2': Could not open '/kvm/iso/test-vdb.qcow2': No such file or directory")

	message := operationErrorMessage("恢复快照失败", err)

	expected := "快照恢复失败：缺少磁盘文件 /kvm/iso/test-vdb.qcow2，请找回文件或删除失效快照"
	if message != expected {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestSnapshotRevertMissingDiskMessageFallback(t *testing.T) {
	message := snapshotRevertMissingDiskMessage("snapshot-revert failed: no such file or directory")

	expected := "快照恢复失败：缺少磁盘文件，请找回文件或删除失效快照"
	if message != expected {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestOperationErrorMessageForMissingMigrationTargetPoolPath(t *testing.T) {
	err := errors.New("target storage pool path for disk /kvm/images/vm1.qcow2 is unavailable")

	message := operationErrorMessage("迁移虚拟机失败", err)

	expected := "目标宿主机没有源磁盘路径 /kvm/images/vm1.qcow2 所在的存储池，无法执行迁移复制磁盘"
	if message != expected {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestOperationErrorMessageForMigrationTargetHostnameLocalhost(t *testing.T) {
	err := errors.New("virsh --connect qemu:///system migrate --live vm1 qemu+ssh://192.168.51.47/system failed: exit status 1: error: internal error: hostname on destination resolved to localhost, but migration requires an FQDN")

	message := operationErrorMessage("迁移虚拟机失败", err)

	expected := "目标宿主机主机名解析为 localhost，热迁移需要目标主机名解析到真实网络地址，请检查目标宿主机 hostname 和 /etc/hosts"
	if message != expected {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestOperationErrorMessageForMigrationNonMigratableDevice(t *testing.T) {
	err := errors.New("virsh --connect qemu:///system migrate --live --unsafe --persistent --auto-converge vm1 qemu+ssh://192.168.51.47/system failed: exit status 1: error: internal error: unable to execute QEMU command 'migrate': State blocked by non-migratable device '0000:00:04.0/ich9_ahci'")

	message := operationErrorMessage("迁移虚拟机失败", err)

	expected := "当前虚拟机包含热迁移不支持的设备 ich9_ahci，请移除或更换该设备后重试，或关机后执行冷迁移"
	if message != expected {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestOperationErrorMessageForSnapshotNonMigratableDevice(t *testing.T) {
	err := errors.New("virsh --connect qemu:///system snapshot-create-as vm1 snap1 --memspec /tmp/vm1.mem failed: exit status 1: error: internal error: State blocked by non-migratable device '0000:00:04.0/ich9_ahci'")

	message := operationErrorMessage("创建快照失败", err)

	expected := "当前虚拟机包含不支持保存运行状态的设备。请取消“包括虚拟机内存”后重试，或调整虚拟硬件后再创建内存快照"
	if message != expected {
		t.Fatalf("unexpected message: %s", message)
	}
}
