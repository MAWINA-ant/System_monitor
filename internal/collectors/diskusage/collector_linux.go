//go:build linux

package diskusage

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/MAWINA-ant/System_monitor/internal/core"
)

const defaultMountsPath = "/proc/mounts"

var _ core.DiskUsageCollector = (*Collector)(nil)

type mountEntry struct {
	device     string
	mountPoint string
	fsType     string
}

type statfsFunc func(path string, stat *syscall.Statfs_t) error

type Collector struct {
	mountsPath string
	statfs     statfsFunc
}

func New() *Collector {
	return &Collector{
		mountsPath: defaultMountsPath,
		statfs:     syscall.Statfs,
	}
}

func (c *Collector) Collect(_ context.Context) ([]core.DiskUsage, error) {
	f, err := os.Open(c.mountsPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", c.mountsPath, err)
	}
	defer f.Close()

	mounts, err := parseMounts(f)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", c.mountsPath, err)
	}

	result := make([]core.DiskUsage, 0, len(mounts))
	for _, m := range mounts {
		var stat syscall.Statfs_t
		if err := c.statfs(m.mountPoint, &stat); err != nil {
			continue
		}

		if stat.Blocks == 0 {
			continue
		}

		result = append(result, buildDiskUsage(m, &stat))
	}

	return result, nil
}

func buildDiskUsage(m mountEntry, stat *syscall.Statfs_t) core.DiskUsage {
	bsize := uint64(stat.Bsize) //nolint:gosec // block size is always non-negative
	total := stat.Blocks * bsize
	free := stat.Bfree * bsize
	avail := stat.Bavail * bsize
	used := total - free

	usedPercent := 0.0
	if used+avail > 0 {
		usedPercent = float64(used) / float64(used+avail) * 100
	}

	usedInodes := stat.Files - stat.Ffree
	usedInodesPercent := 0.0
	if stat.Files > 0 {
		usedInodesPercent = float64(usedInodes) / float64(stat.Files) * 100
	}

	return core.DiskUsage{
		Filesystem:        m.device,
		MountPoint:        m.mountPoint,
		UsedMB:            int64(used / (1024 * 1024)), //nolint:gosec // disk size in MB fits into int64
		UsedPercent:       usedPercent,
		UsedInodes:        int64(usedInodes), //nolint:gosec // inode count fits into int64
		UsedInodesPercent: usedInodesPercent,
	}
}

func parseMounts(r io.Reader) ([]mountEntry, error) {
	var mounts []mountEntry

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		mounts = append(mounts, mountEntry{
			device:     fields[0],
			mountPoint: fields[1],
			fsType:     fields[2],
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return mounts, nil
}
