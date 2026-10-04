//go:build linux

package app

import (
	"testing"

	"github.com/MAWINA-ant/System_monitor/internal/config"
)

func TestNewGRPCServer_RegistersStatsService(t *testing.T) {
	info := NewGRPCServer(config.Default()).GetServiceInfo()

	service, ok := info["statspb.StatsService"]
	if !ok {
		t.Fatalf("statspb.StatsService is not registered, registered: %v", info)
	}

	if len(service.Methods) != 1 || service.Methods[0].Name != "GetStats" || !service.Methods[0].IsServerStream {
		t.Errorf("unexpected methods: %+v", service.Methods)
	}
}
