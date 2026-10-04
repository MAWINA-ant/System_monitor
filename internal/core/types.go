package core

import "time"

type Snapshot struct {
	Timestamp         time.Time
	LoadAverage       *LoadAverage
	CPULoad           *CPULoad
	DiskLoad          []DiskLoad
	DiskUsage         []DiskUsage
	NetworkTopTalkers *NetworkTopTalkers
	NetworkStats      *NetworkStats
}

type LoadAverage struct {
	Load1  float64
	Load5  float64
	Load15 float64
}

type CPULoad struct {
	UserPercent   float64
	SystemPercent float64
	IdlePercent   float64
}

type DiskLoad struct {
	Device   string
	TPS      float64
	KBPerSec float64
}

type DiskUsage struct {
	Filesystem        string
	MountPoint        string
	UsedMB            int64
	UsedPercent       float64
	UsedInodes        int64
	UsedInodesPercent float64
}

type NetworkTopTalkers struct {
	ByProtocol []ProtocolStat
	ByTraffic  []TrafficStat
}

type ProtocolStat struct {
	Protocol string
	Bytes    int64
	Percent  float64
}

type TrafficStat struct {
	Source      string
	Destination string
	Protocol    string
	BytesPerSec float64
}

type NetworkStats struct {
	ListeningSockets []ListeningSocket
	ConnectionStates []ConnectionStateCount
}

type ListeningSocket struct {
	Command  string
	PID      int32
	User     string
	Protocol string
	Port     int32
}

type ConnectionStateCount struct {
	State string
	Count int32
}
