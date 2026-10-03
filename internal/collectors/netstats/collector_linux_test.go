//go:build linux

package netstats

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MAWINA-ant/System_monitor/internal/core"
)

const (
	protoTCP = "tcp"
	userRoot = "root"
)

const tcpFixture = `sl local_address rem_address st tx_queue rx_queue tr tm->when retrnsmt uid timeout inode
0: 00000000:0016 00000000:0000 0A 00000000:00000000 00:00000000 00000000 0 0 111 1
1: 0100007F:1F90 0100007F:D2A0 01 00000000:00000000 00:00000000 00000000 1000 0 222 1
`

const udpFixture = `sl local_address rem_address st tx_queue rx_queue tr tm->when retrnsmt uid timeout inode
0: 00000000:0044 00000000:0000 07 00000000:00000000 00:00000000 00000000 0 0 333 2
`

func TestParseProcNet(t *testing.T) {
	got, err := parseProcNet(strings.NewReader(tcpFixture), protoTCP)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []socketEntry{
		{protocol: protoTCP, port: 22, state: "0A", uid: "0", inode: 111},
		{protocol: protoTCP, port: 8080, state: "01", uid: "1000", inode: 222},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseProcNet() = %+v, want %+v", got, want)
	}
}

func TestParseProcNet_SkipsMalformedLines(t *testing.T) {
	input := "header\n" +
		"garbage line\n" +
		"   0: nocolon 00000000:0000 0A 0:0 0:0 0 0 0 111 1\n" +
		"   1: 00000000:ZZZZ 00000000:0000 0A 0:0 0:0 0 0 0 111 1\n" +
		"   2: 00000000:0050 00000000:0000 0A 0:0 0:0 0 0 0 444 1\n"

	got, err := parseProcNet(strings.NewReader(input), protoTCP)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 || got[0].port != 80 || got[0].inode != 444 {
		t.Errorf("expected only the valid line to survive, got %+v", got)
	}
}

func TestParseSocketLink(t *testing.T) {
	cases := []struct {
		link   string
		inode  uint64
		wantOK bool
	}{
		{"socket:[12345]", 12345, true},
		{"pipe:[999]", 0, false},
		{"/dev/null", 0, false},
		{"socket:[abc]", 0, false},
		{"socket:[123", 0, false},
	}

	for _, tc := range cases {
		inode, ok := parseSocketLink(tc.link)
		if ok != tc.wantOK || inode != tc.inode {
			t.Errorf("parseSocketLink(%q) = (%d, %v), want (%d, %v)", tc.link, inode, ok, tc.inode, tc.wantOK)
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestCollector_Collect(t *testing.T) {
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "net", "tcp"), tcpFixture)
	writeFile(t, filepath.Join(root, "net", "udp"), udpFixture)
	// tcp6/udp6 intentionally absent: IPv6 may be disabled on the host.

	writeFile(t, filepath.Join(root, "123", "comm"), "sshd\n")
	fdDir := filepath.Join(root, "123", "fd")
	if err := os.MkdirAll(fdDir, 0o750); err != nil {
		t.Fatalf("mkdir fd: %v", err)
	}
	if err := os.Symlink("socket:[111]", filepath.Join(fdDir, "3")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	c := &Collector{
		procRoot: root,
		lookupUser: func(uid string) string {
			if uid == "0" {
				return userRoot
			}

			return uid
		},
	}

	got, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := core.NetworkStats{
		ListeningSockets: []core.ListeningSocket{
			{Command: "sshd", PID: 123, User: userRoot, Protocol: protoTCP, Port: 22},
			{Command: "", PID: 0, User: userRoot, Protocol: "udp", Port: 68},
		},
		ConnectionStates: []core.ConnectionStateCount{
			{State: "ESTAB", Count: 1},
			{State: "LISTEN", Count: 1},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Collect() =\n%+v\nwant\n%+v", got, want)
	}
}

func TestCollector_Collect_MissingProcNet(t *testing.T) {
	c := &Collector{procRoot: t.TempDir(), lookupUser: func(uid string) string { return uid }}

	got, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("missing /proc/net files must not be an error, got: %v", err)
	}

	if len(got.ListeningSockets) != 0 || len(got.ConnectionStates) != 0 {
		t.Errorf("expected empty stats, got %+v", got)
	}
}
