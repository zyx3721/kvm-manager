package agent

import "testing"

func TestUserFacingErrorMessageForDuplicateStart(t *testing.T) {
	err := HTTPStatusError{
		StatusCode: 503,
		Status:     "503 Service Unavailable",
		Message:    "virsh --connect qemu:///system start ct7-template failed: exit status 1: error: Failed to start domain 'ct7-template' error: Requested operation is not valid: domain is already running",
	}

	message := UserFacingErrorMessage(err)

	if message != "虚拟机当前已运行" {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestUserFacingErrorMessageForDuplicateShutdown(t *testing.T) {
	err := HTTPStatusError{
		StatusCode: 503,
		Status:     "503 Service Unavailable",
		Message:    "virsh --connect qemu:///system shutdown ct7-template failed: exit status 1: error: Failed to shutdown domain 'ct7-template' error: Requested operation is not valid: domain is not running",
	}

	message := UserFacingErrorMessage(err)

	if message != "虚拟机当前已关机" {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestUserFacingErrorMessageForSuspendNotRunning(t *testing.T) {
	err := HTTPStatusError{
		StatusCode: 503,
		Status:     "503 Service Unavailable",
		Message:    "virsh --connect qemu:///system suspend demo failed: error: Requested operation is not valid: domain is not running",
	}

	message := UserFacingErrorMessage(err)

	if message != "虚拟机当前未运行，无法暂停" {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestUserFacingErrorMessageForResumeNotPaused(t *testing.T) {
	err := HTTPStatusError{
		StatusCode: 503,
		Status:     "503 Service Unavailable",
		Message:    "virsh --connect qemu:///system resume demo failed: error: Requested operation is not valid: domain is not paused",
	}

	message := UserFacingErrorMessage(err)

	if message != "虚拟机当前未暂停，无法恢复" {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestUserFacingErrorMessageForDeleteRunningVM(t *testing.T) {
	err := HTTPStatusError{
		StatusCode: 503,
		Status:     "503 Service Unavailable",
		Message:    "vm 192.168.1.179 must be stopped before delete",
	}

	message := UserFacingErrorMessage(err)

	if message != "请先关闭虚拟机后再删除" {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestUserFacingErrorMessageForDeleteVMWithSnapshots(t *testing.T) {
	err := HTTPStatusError{
		StatusCode: 503,
		Status:     "503 Service Unavailable",
		Message:    "virsh --connect qemu:///system undefine test failed: exit status 1: error: Failed to undefine domain test error: Requested operation is not valid: cannot delete inactive domain with 2 snapshots",
	}

	message := UserFacingErrorMessage(err)

	expected := "删除虚拟机失败：仍存在 2 个快照，请先删除快照后重试"
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

func TestUserFacingErrorMessageForHostResourceLimits(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    string
	}{
		{
			name:    "cpu",
			message: "maximum cpu exceeds host cpu",
			want:    "最大 CPU 不能超过宿主机逻辑 CPU",
		},
		{
			name:    "memory",
			message: "maximum memory exceeds host memory",
			want:    "最大内存不能超过宿主机总内存",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := HTTPStatusError{StatusCode: 503, Status: "503 Service Unavailable", Message: tc.message}
			if got := UserFacingErrorMessage(err); got != tc.want {
				t.Fatalf("unexpected message: got %q want %q", got, tc.want)
			}
		})
	}
}

func TestUserFacingErrorMessageForVMXMLNameMismatch(t *testing.T) {
	err := HTTPStatusError{
		StatusCode: 503,
		Status:     "503 Service Unavailable",
		Message:    "vm xml name mismatch",
	}

	message := UserFacingErrorMessage(err)

	if message != "XML 中的虚拟机名称必须与当前虚拟机名称一致，如需改名请在基本信息中修改" {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestUserFacingErrorMessageForDiskResizeWithInternalSnapshots(t *testing.T) {
	err := HTTPStatusError{
		StatusCode: 503,
		Status:     "503 Service Unavailable",
		Message:    "qemu-img resize /kvm/iso/demo.qcow2 23622320128 failed: exit status 1: qemu-img: Can't resize an image which has snapshots qemu-img: This image does not support resize",
	}

	message := UserFacingErrorMessage(err)

	expected := "磁盘镜像包含内部快照，无法直接扩容。请先删除该虚拟机的内部快照，或重新创建无内部快照的磁盘后再扩容"
	if message != expected {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestUserFacingErrorMessageForSnapshotRevertMissingDisk(t *testing.T) {
	err := HTTPStatusError{
		StatusCode: 503,
		Status:     "503 Service Unavailable",
		Message:    "virsh --connect qemu:///system snapshot-revert test snap-test-202606040933 failed: exit status 1: error: internal error: Child process (/usr/bin/qemu-img snapshot -a snap-test-202606040933 /kvm/iso/test-vdb.qcow2) unexpected exit status 1: qemu-img: Could not open '/kvm/iso/test-vdb.qcow2': Could not open '/kvm/iso/test-vdb.qcow2': No such file or directory",
	}

	message := UserFacingErrorMessage(err)

	expected := "快照恢复失败：缺少磁盘文件 /kvm/iso/test-vdb.qcow2，请找回文件或删除失效快照"
	if message != expected {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestUserFacingErrorMessageForMigrationMissingStorageFile(t *testing.T) {
	err := HTTPStatusError{
		StatusCode: 503,
		Status:     "503 Service Unavailable",
		Message:    "virsh --connect qemu:///system migrate --live --persistent --auto-converge vm1 qemu+ssh://192.168.51.47/system failed: exit status 1: error: Cannot access storage file '/data/kvm/ct7.qcow2' (as uid:0, gid:0): No such file or directory",
	}

	message := UserFacingErrorMessage(err)

	expected := "目标宿主机无法访问源磁盘路径 /data/kvm/ct7.qcow2，请确认目标宿主机存在该路径所在目录或存储池后重试"
	if message != expected {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestUserFacingErrorMessageForMigrationGuestMemoryAllocation(t *testing.T) {
	err := HTTPStatusError{
		StatusCode: 503,
		Status:     "503 Service Unavailable",
		Message:    "virsh --connect qemu:///system migrate --live --persistent --auto-converge vm1 qemu+ssh://192.168.51.47/system failed: exit status 1: error: internal error: qemu unexpectedly closed the monitor: Cannot set up guest memory 'pc.ram': Cannot allocate memory",
	}

	message := UserFacingErrorMessage(err)

	expected := "目标宿主机可用内存不足，无法为迁移虚拟机分配内存，请释放目标宿主机内存或降低虚拟机内存后重试"
	if message != expected {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestUserFacingErrorMessageKeepsStartupMissingStorageFile(t *testing.T) {
	err := HTTPStatusError{
		StatusCode: 503,
		Status:     "503 Service Unavailable",
		Message:    "virsh --connect qemu:///system start ct7-template failed: exit status 1: error: Failed to start domain 'ct7-template' error: Cannot access storage file '/kvm/iso/CentOS-7.9-x86_64-DVD-2009.iso': No such file or directory",
	}

	message := UserFacingErrorMessage(err)

	if message != err.Message {
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

func TestUserFacingErrorMessageForMissingMigrationTargetPoolPath(t *testing.T) {
	err := HTTPStatusError{
		StatusCode: 503,
		Status:     "503 Service Unavailable",
		Message:    "target storage pool path for disk /kvm/images/vm1.qcow2 is unavailable",
	}

	message := UserFacingErrorMessage(err)

	expected := "目标宿主机没有源磁盘路径 /kvm/images/vm1.qcow2 所在的存储池，无法执行迁移复制磁盘"
	if message != expected {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestUserFacingErrorMessageForMigrationTargetHostnameLocalhost(t *testing.T) {
	err := HTTPStatusError{
		StatusCode: 503,
		Status:     "503 Service Unavailable",
		Message:    "hostname on destination resolved to localhost, but migration requires an FQDN",
	}

	message := UserFacingErrorMessage(err)

	expected := "目标宿主机主机名解析为 localhost，热迁移需要目标主机名解析到真实网络地址，请检查目标宿主机 hostname 和 /etc/hosts"
	if message != expected {
		t.Fatalf("unexpected message: %s", message)
	}
}
