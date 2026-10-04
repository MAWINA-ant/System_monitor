package server

import (
	"testing"
	"time"

	pb "github.com/MAWINA-ant/System_monitor/api/statspb"
	"github.com/MAWINA-ant/System_monitor/internal/core"
	"google.golang.org/protobuf/proto"
)

const protoTCP = "tcp"

func TestToProto_AllSections(t *testing.T) {
	snap := core.Snapshot{
		Timestamp:   time.Unix(1700000000, 0),
		LoadAverage: &core.LoadAverage{Load1: 0.5, Load5: 0.4, Load15: 0.3},
		CPULoad:     &core.CPULoad{UserPercent: 10, SystemPercent: 5, IdlePercent: 85},
		DiskLoad:    []core.DiskLoad{{Device: "sda", TPS: 50, KBPerSec: 1000}},
		DiskUsage: []core.DiskUsage{{
			Filesystem: "/dev/sda1", MountPoint: "/", UsedMB: 100, UsedPercent: 40,
			UsedInodes: 7, UsedInodesPercent: 3,
		}},
		NetworkTopTalkers: &core.NetworkTopTalkers{
			ByProtocol: []core.ProtocolStat{{Protocol: protoTCP, Bytes: 900, Percent: 90}},
			ByTraffic: []core.TrafficStat{{
				Source: "1.1.1.1:1", Destination: "2.2.2.2:2", Protocol: protoTCP, BytesPerSec: 12.5,
			}},
		},
		NetworkStats: &core.NetworkStats{
			ListeningSockets: []core.ListeningSocket{{Command: "sshd", PID: 1, User: "root", Protocol: protoTCP, Port: 22}},
			ConnectionStates: []core.ConnectionStateCount{{State: "ESTAB", Count: 3}},
		},
	}

	want := &pb.Snapshot{
		Timestamp:   1700000000,
		LoadAverage: &pb.LoadAverage{Load1: 0.5, Load5: 0.4, Load15: 0.3},
		CpuLoad:     &pb.CPULoad{UserPercent: 10, SystemPercent: 5, IdlePercent: 85},
		DiskLoad:    []*pb.DiskLoad{{Device: "sda", Tps: 50, KbPerSec: 1000}},
		DiskUsage: []*pb.DiskUsage{{
			Filesystem: "/dev/sda1", MountPoint: "/", UsedMb: 100, UsedPercent: 40,
			UsedInodes: 7, UsedInodesPercent: 3,
		}},
		NetworkTopTalkers: &pb.NetworkTopTalkers{
			ByProtocol: []*pb.ProtocolStat{{Protocol: protoTCP, Bytes: 900, Percent: 90}},
			ByTraffic: []*pb.TrafficStat{{
				Source: "1.1.1.1:1", Destination: "2.2.2.2:2", Protocol: protoTCP, BytesPerSec: 12.5,
			}},
		},
		NetworkStats: &pb.NetworkStats{
			ListeningSockets: []*pb.ListeningSocket{{Command: "sshd", Pid: 1, User: "root", Protocol: protoTCP, Port: 22}},
			ConnectionStates: []*pb.ConnectionStateCount{{State: "ESTAB", Count: 3}},
		},
	}

	if got := toProto(snap); !proto.Equal(got, want) {
		t.Errorf("toProto() =\n%v\nwant\n%v", got, want)
	}
}

func TestToProto_DisabledSectionsStayAbsent(t *testing.T) {
	got := toProto(core.Snapshot{Timestamp: time.Unix(5, 0)})

	if got.GetLoadAverage() != nil || got.GetCpuLoad() != nil ||
		got.GetNetworkTopTalkers() != nil || got.GetNetworkStats() != nil {
		t.Errorf("disabled subsystems must not be present in the message: %v", got)
	}
	if got.GetTimestamp() != 5 {
		t.Errorf("Timestamp = %d, want 5", got.GetTimestamp())
	}
}
