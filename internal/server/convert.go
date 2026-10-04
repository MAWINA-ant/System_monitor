package server

import (
	pb "github.com/MAWINA-ant/System_monitor/api/statspb"
	"github.com/MAWINA-ant/System_monitor/internal/core"
)

func toProto(s core.Snapshot) *pb.Snapshot {
	return &pb.Snapshot{
		Timestamp:         s.Timestamp.Unix(),
		LoadAverage:       loadAverageToProto(s.LoadAverage),
		CpuLoad:           cpuLoadToProto(s.CPULoad),
		DiskLoad:          diskLoadToProto(s.DiskLoad),
		DiskUsage:         diskUsageToProto(s.DiskUsage),
		NetworkTopTalkers: topTalkersToProto(s.NetworkTopTalkers),
		NetworkStats:      networkStatsToProto(s.NetworkStats),
	}
}

func loadAverageToProto(la *core.LoadAverage) *pb.LoadAverage {
	if la == nil {
		return nil
	}

	return &pb.LoadAverage{Load1: la.Load1, Load5: la.Load5, Load15: la.Load15}
}

func cpuLoadToProto(c *core.CPULoad) *pb.CPULoad {
	if c == nil {
		return nil
	}

	return &pb.CPULoad{
		UserPercent:   c.UserPercent,
		SystemPercent: c.SystemPercent,
		IdlePercent:   c.IdlePercent,
	}
}

func diskLoadToProto(disks []core.DiskLoad) []*pb.DiskLoad {
	result := make([]*pb.DiskLoad, 0, len(disks))
	for _, d := range disks {
		result = append(result, &pb.DiskLoad{Device: d.Device, Tps: d.TPS, KbPerSec: d.KBPerSec})
	}

	return result
}

func diskUsageToProto(disks []core.DiskUsage) []*pb.DiskUsage {
	result := make([]*pb.DiskUsage, 0, len(disks))
	for _, d := range disks {
		result = append(result, &pb.DiskUsage{
			Filesystem:        d.Filesystem,
			MountPoint:        d.MountPoint,
			UsedMb:            d.UsedMB,
			UsedPercent:       d.UsedPercent,
			UsedInodes:        d.UsedInodes,
			UsedInodesPercent: d.UsedInodesPercent,
		})
	}

	return result
}

func topTalkersToProto(t *core.NetworkTopTalkers) *pb.NetworkTopTalkers {
	if t == nil {
		return nil
	}

	result := &pb.NetworkTopTalkers{
		ByProtocol: make([]*pb.ProtocolStat, 0, len(t.ByProtocol)),
		ByTraffic:  make([]*pb.TrafficStat, 0, len(t.ByTraffic)),
	}

	for _, p := range t.ByProtocol {
		result.ByProtocol = append(result.ByProtocol, &pb.ProtocolStat{
			Protocol: p.Protocol,
			Bytes:    p.Bytes,
			Percent:  p.Percent,
		})
	}

	for _, f := range t.ByTraffic {
		result.ByTraffic = append(result.ByTraffic, &pb.TrafficStat{
			Source:      f.Source,
			Destination: f.Destination,
			Protocol:    f.Protocol,
			BytesPerSec: f.BytesPerSec,
		})
	}

	return result
}

func networkStatsToProto(n *core.NetworkStats) *pb.NetworkStats {
	if n == nil {
		return nil
	}

	result := &pb.NetworkStats{
		ListeningSockets: make([]*pb.ListeningSocket, 0, len(n.ListeningSockets)),
		ConnectionStates: make([]*pb.ConnectionStateCount, 0, len(n.ConnectionStates)),
	}

	for _, s := range n.ListeningSockets {
		result.ListeningSockets = append(result.ListeningSockets, &pb.ListeningSocket{
			Command:  s.Command,
			Pid:      s.PID,
			User:     s.User,
			Protocol: s.Protocol,
			Port:     s.Port,
		})
	}

	for _, c := range n.ConnectionStates {
		result.ConnectionStates = append(result.ConnectionStates, &pb.ConnectionStateCount{
			State: c.State,
			Count: c.Count,
		})
	}

	return result
}
