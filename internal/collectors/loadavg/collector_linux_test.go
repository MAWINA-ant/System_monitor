//go:build linux

package loadavg

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/MAWINA-ant/System_monitor/internal/core"
)

func TestCollector_Collect(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loadavg")
	content := "0.52 0.58 0.59 2/543 12345\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	c := &Collector{path: path}

	got, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := core.LoadAverage{Load1: 0.52, Load5: 0.58, Load15: 0.59}
	if got != want {
		t.Errorf("Collect() = %+v, want %+v", got, want)
	}
}

func TestCollector_Collect_InvalidFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loadavg")
	if err := os.WriteFile(path, []byte("garbage"), 0o600); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	c := &Collector{path: path}

	if _, err := c.Collect(context.Background()); err == nil {
		t.Fatal("expected error for invalid format, got nil")
	}
}

func TestCollector_Collect_FileNotFound(t *testing.T) {
	c := &Collector{path: "/nonexistent/path"}

	if _, err := c.Collect(context.Background()); err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
