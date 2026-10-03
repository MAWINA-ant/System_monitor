//go:build linux

package netstats

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/MAWINA-ant/System_monitor/internal/core"
)

const (
	defaultProcRoot = "/proc"

	stateListen     = "0A"
	stateUDPUnconn  = "07"
	unknownStateStr = "UNKNOWN"
)

var _ core.NetworkStatsCollector = (*Collector)(nil)

var procNetFiles = []struct {
	file     string
	protocol string
}{
	{"tcp", "tcp"},
	{"tcp6", "tcp6"},
	{"udp", "udp"},
	{"udp6", "udp6"},
}

var tcpStateNames = map[string]string{
	"01": "ESTAB",
	"02": "SYN_SENT",
	"03": "SYN_RECV",
	"04": "FIN_WAIT1",
	"05": "FIN_WAIT2",
	"06": "TIME_WAIT",
	"07": "CLOSE",
	"08": "CLOSE_WAIT",
	"09": "LAST_ACK",
	"0A": "LISTEN",
	"0B": "CLOSING",
}

type socketEntry struct {
	protocol string
	port     int32
	state    string
	uid      string
	inode    uint64
}

type procInfo struct {
	pid     int32
	command string
}

type Collector struct {
	procRoot   string
	lookupUser func(uid string) string
}

func New() *Collector {
	return &Collector{procRoot: defaultProcRoot, lookupUser: lookupUserName}
}

func (c *Collector) Collect(_ context.Context) (core.NetworkStats, error) {
	inodes := c.buildInodeMap()
	users := make(map[string]string)
	stateCounts := make(map[string]int32)
	var listening []core.ListeningSocket

	for _, src := range procNetFiles {
		entries, err := c.readProcNet(src.file, src.protocol)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return core.NetworkStats{}, err
		}

		for _, e := range entries {
			if isListening(e) {
				listening = append(listening, c.toListeningSocket(e, inodes, users))
			}
			if strings.HasPrefix(e.protocol, "tcp") {
				stateCounts[tcpStateName(e.state)]++
			}
		}
	}

	return core.NetworkStats{
		ListeningSockets: sortListening(listening),
		ConnectionStates: sortStates(stateCounts),
	}, nil
}

func (c *Collector) readProcNet(file, protocol string) ([]socketEntry, error) {
	path := filepath.Join(c.procRoot, "net", file)

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	entries, err := parseProcNet(f, protocol)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	return entries, nil
}

func (c *Collector) toListeningSocket(
	e socketEntry,
	inodes map[uint64]procInfo,
	users map[string]string,
) core.ListeningSocket {
	user, ok := users[e.uid]
	if !ok {
		user = c.lookupUser(e.uid)
		users[e.uid] = user
	}

	info := inodes[e.inode]

	return core.ListeningSocket{
		Command:  info.command,
		PID:      info.pid,
		User:     user,
		Protocol: e.protocol,
		Port:     e.port,
	}
}

func isListening(e socketEntry) bool {
	if strings.HasPrefix(e.protocol, "tcp") {
		return e.state == stateListen
	}

	return e.state == stateUDPUnconn
}

func tcpStateName(code string) string {
	if name, ok := tcpStateNames[code]; ok {
		return name
	}

	return unknownStateStr
}

func sortListening(sockets []core.ListeningSocket) []core.ListeningSocket {
	sort.Slice(sockets, func(i, j int) bool {
		if sockets[i].Protocol != sockets[j].Protocol {
			return sockets[i].Protocol < sockets[j].Protocol
		}

		return sockets[i].Port < sockets[j].Port
	})

	return sockets
}

func sortStates(counts map[string]int32) []core.ConnectionStateCount {
	result := make([]core.ConnectionStateCount, 0, len(counts))
	for state, count := range counts {
		result = append(result, core.ConnectionStateCount{State: state, Count: count})
	}

	sort.Slice(result, func(i, j int) bool { return result[i].State < result[j].State })

	return result
}

func parseProcNet(r io.Reader, protocol string) ([]socketEntry, error) {
	var entries []socketEntry

	scanner := bufio.NewScanner(r)
	scanner.Scan() // header line

	for scanner.Scan() {
		if e, ok := parseProcNetLine(scanner.Text(), protocol); ok {
			entries = append(entries, e)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

func parseProcNetLine(line, protocol string) (socketEntry, bool) {
	fields := strings.Fields(line)
	if len(fields) < 10 {
		return socketEntry{}, false
	}

	_, portHex, found := strings.Cut(fields[1], ":")
	if !found {
		return socketEntry{}, false
	}

	port, err := strconv.ParseInt(portHex, 16, 32)
	if err != nil {
		return socketEntry{}, false
	}

	inode, err := strconv.ParseUint(fields[9], 10, 64)
	if err != nil {
		return socketEntry{}, false
	}

	return socketEntry{
		protocol: protocol,
		port:     int32(port),
		state:    strings.ToUpper(fields[3]),
		uid:      fields[7],
		inode:    inode,
	}, true
}

func (c *Collector) buildInodeMap() map[uint64]procInfo {
	result := make(map[uint64]procInfo)

	entries, err := os.ReadDir(c.procRoot)
	if err != nil {
		return result
	}

	for _, e := range entries {
		pid, err := strconv.ParseInt(e.Name(), 10, 32)
		if err != nil {
			continue
		}

		c.scanProcess(e.Name(), int32(pid), result)
	}

	return result
}

func (c *Collector) scanProcess(dirName string, pid int32, result map[uint64]procInfo) {
	fdDir := filepath.Join(c.procRoot, dirName, "fd")

	fds, err := os.ReadDir(fdDir)
	if err != nil {
		return
	}

	var command string
	commandRead := false

	for _, fd := range fds {
		link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
		if err != nil {
			continue
		}

		inode, ok := parseSocketLink(link)
		if !ok {
			continue
		}

		if _, exists := result[inode]; exists {
			continue
		}

		if !commandRead {
			command = c.readCommand(dirName)
			commandRead = true
		}

		result[inode] = procInfo{pid: pid, command: command}
	}
}

func (c *Collector) readCommand(dirName string) string {
	data, err := os.ReadFile(filepath.Join(c.procRoot, dirName, "comm"))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(data))
}

func parseSocketLink(link string) (uint64, bool) {
	rest, ok := strings.CutPrefix(link, "socket:[")
	if !ok {
		return 0, false
	}

	rest, ok = strings.CutSuffix(rest, "]")
	if !ok {
		return 0, false
	}

	inode, err := strconv.ParseUint(rest, 10, 64)
	if err != nil {
		return 0, false
	}

	return inode, true
}

func lookupUserName(uid string) string {
	u, err := user.LookupId(uid)
	if err != nil {
		return uid
	}

	return u.Username
}
