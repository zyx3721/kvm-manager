package kvm

import "testing"

func TestSummarizeLocalDiskFilesystems(t *testing.T) {
	rows := parseDFStorageRows(`Filesystem     1B-blocks Used Available Use% Mounted on
/dev/sda1      100       40   60        40% /
tmpfs          200       10   190       5% /run
overlay        300       20   280       7% /var/lib/docker/overlay2/demo
/dev/sdb1      400       100  300       25% /data
/dev/sdb1      400       100  300       25% /data-bind
`)
	total, used := summarizeLocalDiskFilesystems(rows)

	if total != 500 {
		t.Fatalf("expected local disk filesystems total 500, got %d", total)
	}
	if used != 140 {
		t.Fatalf("expected local disk filesystems used 140, got %d", used)
	}
}

func TestParseDFStorageRowsSkipsHeaderAndMalformedRows(t *testing.T) {
	rows := parseDFStorageRows(`Filesystem     1B-blocks Used Available Use% Mounted on
malformed
/dev/sda1      100       40   60        40% /
`)

	if len(rows) != 1 {
		t.Fatalf("expected one parsed row, got %d", len(rows))
	}
	if rows[0].Filesystem != "/dev/sda1" || rows[0].Total != 100 || rows[0].Used != 40 || rows[0].MountedOn != "/" {
		t.Fatalf("unexpected row: %+v", rows[0])
	}
}
