package client

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	pb "github.com/MAWINA-ant/System_monitor/api/statspb"
)

const (
	noData   = "no data"
	disabled = "disabled on the server"
	dash     = "-"
)

type column struct {
	title      string
	alignRight bool
}

// Format renders the requested sections of a snapshot as plain text with aligned tables.
func Format(s *pb.Snapshot, sections Sections) string {
	var b strings.Builder

	b.WriteString("=== " + time.Unix(s.GetTimestamp(), 0).Format("2006-01-02 15:04:05") + " ===\n")

	if sections[SectionLoad] {
		b.WriteString(formatLoad(s.GetLoadAverage()))
	}
	if sections[SectionCPU] {
		b.WriteString(formatCPU(s.GetCpuLoad()))
	}
	if sections[SectionDiskLoad] {
		b.WriteString(formatDiskLoad(s.GetDiskLoad()))
	}
	if sections[SectionDiskUsage] {
		b.WriteString(formatDiskUsage(s.GetDiskUsage()))
	}
	if sections[SectionTopTalkers] {
		b.WriteString(formatTopTalkers(s.GetNetworkTopTalkers()))
	}
	if sections[SectionNetStats] {
		b.WriteString(formatNetStats(s.GetNetworkStats()))
	}

	return b.String()
}

func formatLoad(la *pb.LoadAverage) string {
	if la == nil {
		return "Load average: " + disabled + "\n"
	}

	return fmt.Sprintf("Load average: %.2f %.2f %.2f (1/5/15 min)\n", la.GetLoad1(), la.GetLoad5(), la.GetLoad15())
}

func formatCPU(c *pb.CPULoad) string {
	if c == nil {
		return "CPU: " + disabled + "\n"
	}

	return fmt.Sprintf("CPU: user %.1f%%  system %.1f%%  idle %.1f%%\n",
		c.GetUserPercent(), c.GetSystemPercent(), c.GetIdlePercent())
}

func formatDiskLoad(disks []*pb.DiskLoad) string {
	rows := make([][]string, 0, len(disks))
	for _, d := range disks {
		rows = append(rows, []string{d.GetDevice(), f2(d.GetTps()), f2(d.GetKbPerSec())})
	}

	return section("Disk load", []column{{"DEVICE", false}, {"TPS", true}, {"KB/S", true}}, rows)
}

func formatDiskUsage(disks []*pb.DiskUsage) string {
	rows := make([][]string, 0, len(disks))
	for _, d := range disks {
		rows = append(rows, []string{
			d.GetFilesystem(), d.GetMountPoint(),
			strconv.FormatInt(d.GetUsedMb(), 10), f1(d.GetUsedPercent()),
			strconv.FormatInt(d.GetUsedInodes(), 10), f1(d.GetUsedInodesPercent()),
		})
	}

	columns := []column{
		{"FILESYSTEM", false},
		{"MOUNTED ON", false},
		{"USED MB", true},
		{"USED %", true},
		{"INODES", true},
		{"INODES %", true},
	}

	return section("Disk usage", columns, rows)
}

func formatTopTalkers(t *pb.NetworkTopTalkers) string {
	if t == nil {
		return "\nTop talkers: " + disabled + "\n"
	}

	protocols := make([][]string, 0, len(t.GetByProtocol()))
	for _, p := range t.GetByProtocol() {
		protocols = append(protocols, []string{p.GetProtocol(), strconv.FormatInt(p.GetBytes(), 10), f1(p.GetPercent())})
	}

	flows := make([][]string, 0, len(t.GetByTraffic()))
	for _, f := range t.GetByTraffic() {
		flows = append(flows, []string{f.GetSource(), f.GetDestination(), f.GetProtocol(), f2(f.GetBytesPerSec())})
	}

	return section("Top talkers by protocol",
		[]column{{"PROTOCOL", false}, {"BYTES", true}, {"%", true}}, protocols) +
		section("Top talkers by traffic",
			[]column{{"SOURCE", false}, {"DESTINATION", false}, {"PROTOCOL", false}, {"BYTES/S", true}}, flows)
}

func formatNetStats(n *pb.NetworkStats) string {
	if n == nil {
		return "\nNetwork: " + disabled + "\n"
	}

	sockets := make([][]string, 0, len(n.GetListeningSockets()))
	for _, s := range n.GetListeningSockets() {
		pid := dash
		if s.GetPid() != 0 {
			pid = strconv.Itoa(int(s.GetPid()))
		}

		sockets = append(sockets, []string{
			s.GetProtocol(), strconv.Itoa(int(s.GetPort())), pid, orDash(s.GetUser()), orDash(s.GetCommand()),
		})
	}

	states := make([][]string, 0, len(n.GetConnectionStates()))
	for _, c := range n.GetConnectionStates() {
		states = append(states, []string{c.GetState(), strconv.Itoa(int(c.GetCount()))})
	}

	return section("Listening sockets",
		[]column{{"PROTO", false}, {"PORT", true}, {"PID", true}, {"USER", false}, {"COMMAND", false}}, sockets) +
		section("TCP connections by state", []column{{"STATE", false}, {"COUNT", true}}, states)
}

func section(title string, columns []column, rows [][]string) string {
	if len(rows) == 0 {
		return "\n" + title + ": " + noData + "\n"
	}

	return "\n" + title + "\n" + table(columns, rows)
}

// table aligns columns by the widest cell, numbers go to the right.
func table(columns []column, rows [][]string) string {
	widths := make([]int, len(columns))
	for i, c := range columns {
		widths[i] = len(c.title)
	}
	for _, row := range rows {
		for i, cell := range row {
			widths[i] = max(widths[i], len(cell))
		}
	}

	var b strings.Builder

	writeRow := func(cells []string) {
		for i, cell := range cells {
			pad := strings.Repeat(" ", widths[i]-len(cell))

			switch {
			case columns[i].alignRight:
				b.WriteString(pad + cell)
			case i == len(cells)-1:
				b.WriteString(cell)
			default:
				b.WriteString(cell + pad)
			}

			if i < len(cells)-1 {
				b.WriteString("  ")
			}
		}
		b.WriteString("\n")
	}

	titles := make([]string, len(columns))
	for i, c := range columns {
		titles[i] = c.title
	}

	writeRow(titles)
	for _, row := range rows {
		writeRow(row)
	}

	return b.String()
}

func f1(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }

func f2(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

func orDash(s string) string {
	if s == "" {
		return dash
	}

	return s
}
