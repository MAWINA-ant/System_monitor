//go:build linux

package loadavg

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/MAWINA-ant/System_monitor/internal/core"
)

const defaultPath = "/proc/loadavg"

var _ core.LoadAverageCollector = (*Collector)(nil)

type Collector struct {
	path string
}

func New() *Collector {
	return &Collector{path: defaultPath}
}

func (c *Collector) Collect(_ context.Context) (core.LoadAverage, error) {
	data, err := os.ReadFile(c.path)
	if err != nil {
		return core.LoadAverage{}, fmt.Errorf("read %s: %w", c.path, err)
	}

	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return core.LoadAverage{}, fmt.Errorf("unexpected format of %s: %q", c.path, data)
	}

	load1, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return core.LoadAverage{}, fmt.Errorf("parse load1: %w", err)
	}
	load5, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return core.LoadAverage{}, fmt.Errorf("parse load5: %w", err)
	}
	load15, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return core.LoadAverage{}, fmt.Errorf("parse load15: %w", err)
	}

	return core.LoadAverage{Load1: load1, Load5: load5, Load15: load15}, nil
}
