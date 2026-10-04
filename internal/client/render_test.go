package client

import (
	"strings"
	"testing"
	"time"

	pb "github.com/MAWINA-ant/System_monitor/api/statspb"
)

const (
	testTimestamp = 1700000000
	protoTCP      = "tcp"
)

func header() string {
	return "=== " + time.Unix(testTimestamp, 0).Format("2006-01-02 15:04:05") + " ===\n"
}

func fullSnapshot() *pb.Snapshot {
	return &pb.Snapshot{
		Timestamp:   testTimestamp,
		LoadAverage: &pb.LoadAverage{Load1: 0.5, Load5: 0.4, Load15: 0.3},
		CpuLoad:     &pb.CPULoad{UserPercent: 10, SystemPercent: 5, IdlePercent: 85},
		DiskLoad:    []*pb.DiskLoad{{Device: "sda", Tps: 50, KbPerSec: 1000}, {Device: "nvme0n1", Tps: 1.5, KbPerSec: 20}},
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
			ListeningSockets: []*pb.ListeningSocket{
				{Command: "sshd", Pid: 1, User: "root", Protocol: protoTCP, Port: 22},
				{Protocol: "udp", Port: 68},
			},
			ConnectionStates: []*pb.ConnectionStateCount{{State: "ESTAB", Count: 3}},
		},
	}
}

func TestFormat_AllSections(t *testing.T) {
	want := header() + `Load average: 0.50 0.40 0.30 (1/5/15 min)
CPU: user 10.0%  system 5.0%  idle 85.0%

Disk load
DEVICE     TPS     KB/S
sda      50.00  1000.00
nvme0n1   1.50    20.00

Disk usage
FILESYSTEM  MOUNTED ON  USED MB  USED %  INODES  INODES %
/dev/sda1   /               100    40.0       7       3.0

Top talkers by protocol
PROTOCOL  BYTES     %
tcp         900  90.0

Top talkers by traffic
SOURCE     DESTINATION  PROTOCOL  BYTES/S
1.1.1.1:1  2.2.2.2:2    tcp         12.50

Listening sockets
PROTO  PORT  PID  USER  COMMAND
tcp      22    1  root  sshd
udp      68    -  -     -

TCP connections by state
STATE  COUNT
ESTAB      3
`

	got := Format(fullSnapshot(), allSectionsSet())
	if got != want {
		t.Errorf("Format() =\n%s\nwant\n%s", got, want)
	}
}

func TestFormat_OnlySelectedSections(t *testing.T) {
	got := Format(fullSnapshot(), Sections{SectionCPU: true})

	want := header() + "CPU: user 10.0%  system 5.0%  idle 85.0%\n"
	if got != want {
		t.Errorf("Format() = %q, want %q", got, want)
	}
}

func TestFormat_DisabledAndEmptySections(t *testing.T) {
	got := Format(&pb.Snapshot{Timestamp: testTimestamp}, allSectionsSet())

	for _, want := range []string{
		"Load average: disabled on the server",
		"CPU: disabled on the server",
		"Disk load: no data",
		"Disk usage: no data",
		"Top talkers: disabled on the server",
		"Network: disabled on the server",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not contain %q:\n%s", want, got)
		}
	}
}

func TestFormat_EmptyTablesOfEnabledSubsystem(t *testing.T) {
	snap := &pb.Snapshot{
		Timestamp:         testTimestamp,
		NetworkTopTalkers: &pb.NetworkTopTalkers{},
		NetworkStats:      &pb.NetworkStats{},
	}

	got := Format(snap, Sections{SectionTopTalkers: true, SectionNetStats: true})

	for _, want := range []string{
		"Top talkers by protocol: no data",
		"Top talkers by traffic: no data",
		"Listening sockets: no data",
		"TCP connections by state: no data",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not contain %q:\n%s", want, got)
		}
	}
}
