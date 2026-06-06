package realtime

import "testing"

func TestRecordThresholdAlertSampleRequiresConsecutiveHits(t *testing.T) {
	service := New(nil, nil, "", nil, nil, 1, 0)

	if got := service.recordThresholdAlertSample("virtual_machine", "vm-1:memory", "虚拟机内存使用率过高", true); got != 1 {
		t.Fatalf("expected first hit count 1, got %d", got)
	}
	if got := service.recordThresholdAlertSample("virtual_machine", "vm-1:memory", "虚拟机内存使用率过高", true); got != 2 {
		t.Fatalf("expected second hit count 2, got %d", got)
	}
	if got := service.recordThresholdAlertSample("virtual_machine", "vm-1:memory", "虚拟机内存使用率过高", true); got != alertConsecutiveLimit {
		t.Fatalf("expected third hit count %d, got %d", alertConsecutiveLimit, got)
	}
	if got := service.recordThresholdAlertSample("virtual_machine", "vm-1:memory", "虚拟机内存使用率过高", false); got != 0 {
		t.Fatalf("expected reset count 0, got %d", got)
	}
	if got := service.recordThresholdAlertSample("virtual_machine", "vm-1:memory", "虚拟机内存使用率过高", true); got != 1 {
		t.Fatalf("expected count to restart at 1, got %d", got)
	}
}
