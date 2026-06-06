package router

import "testing"

func TestFriendlyStorageVolumeMessageInUseVolume(t *testing.T) {
	input := `qemu-img convert failed: exit status 1: qemu-img: Failed to get shared "write" lock`
	got := friendlyStorageVolumeMessage(input)
	want := "当前镜像正在被虚拟机使用，无法克隆。请先关闭相关虚拟机，或通过快照创建一致性副本"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestFriendlyStorageVolumeMessageExistingVolume(t *testing.T) {
	input := "error: storage volume 'test.img' exists already"
	got := friendlyStorageVolumeMessage(input)
	want := "镜像名称已存在，请更换名称"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
