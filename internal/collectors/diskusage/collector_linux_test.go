//go:build linux

package diskusage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestParseMounts(t *testing.T) {
	input := "/dev/sda1 / ext4 rw,relatime 0 0\n" +
		"proc /proc proc rw,nosuid 0 0\n" +
		"tmpfs /run tmpfs rw,nosuid,size=100000k 0 0\n"

	got, err := parseMounts(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []mountEntry{
		{device: "/dev/sda1", mountPoint: "/", fsType: "ext4"},
		{device: "proc", mountPoint: "/proc", fsType: "proc"},
		{device: "tmpfs", mountPoint: "/run", fsType: "tmpfs"},
	}

	if len(got) != len(want) {
		t.Fatalf("parseMounts() returned %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestCollector_Collect(t *testing.T) {
	dir := t.TempDir()
	mountsPath := filepath.Join(dir, "mounts")
	content := "/dev/sda1 / ext4 rw,relatime 0 0\nproc /proc proc rw,nosuid 0 0\n"
	if err := os.WriteFile(mountsPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	c := &Collector{
		mountsPath: mountsPath,
		statfs: func(path string, stat *syscall.Statfs_t) error {
			switch path {
			case "/":
				*stat = syscall.Statfs_t{
					Bsize:  4096,
					Blocks: 1000,
					Bfree:  400,
					Bavail: 350,
					Files:  100,
					Ffree:  60,
				}
			case "/proc":
				*stat = syscall.Statfs_t{}
			}
			return nil
		},
	}

	got, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("Collect() returned %d entries, want 1 (pseudo-fs should be skipped): %+v", len(got), got)
	}

	du := got[0]
	if du.Filesystem != "/dev/sda1" || du.MountPoint != "/" {
		t.Errorf("unexpected entry: %+v", du)
	}

	wantUsedMB := int64((1000 - 400) * 4096 / (1024 * 1024))
	if du.UsedMB != wantUsedMB {
		t.Errorf("UsedMB = %d, want %d", du.UsedMB, wantUsedMB)
	}

	wantUsedPercent := float64(1000-400) / float64(1000-400+350) * 100
	if du.UsedPercent != wantUsedPercent {
		t.Errorf("UsedPercent = %v, want %v", du.UsedPercent, wantUsedPercent)
	}

	wantUsedInodes := int64(100 - 60)
	if du.UsedInodes != wantUsedInodes {
		t.Errorf("UsedInodes = %d, want %d", du.UsedInodes, wantUsedInodes)
	}
}

func TestCollector_Collect_StatfsErrorSkipsEntry(t *testing.T) {
	dir := t.TempDir()
	mountsPath := filepath.Join(dir, "mounts")
	content := "/dev/sda1 / ext4 rw,relatime 0 0\n/dev/sdb1 /data ext4 rw,relatime 0 0\n"
	if err := os.WriteFile(mountsPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	c := &Collector{
		mountsPath: mountsPath,
		statfs: func(path string, stat *syscall.Statfs_t) error {
			if path == "/data" {
				return syscall.ENOENT
			}
			*stat = syscall.Statfs_t{Bsize: 4096, Blocks: 500, Bfree: 200, Bavail: 200, Files: 50, Ffree: 10}
			return nil
		},
	}

	got, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1", len(got))
	}
	if got[0].MountPoint != "/" {
		t.Errorf("unexpected surviving entry: %+v", got[0])
	}
}
