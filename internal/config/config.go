package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

const (
	defaultPort = 8080
	maxPort     = 65535
)

// Subsystems tells which statistics the daemon collects.
type Subsystems struct {
	LoadAverage  bool `json:"loadAverage"`
	CPU          bool `json:"cpu"`
	DiskLoad     bool `json:"diskLoad"`
	DiskUsage    bool `json:"diskUsage"`
	NetworkStats bool `json:"networkStats"`
}

type Config struct {
	Subsystems Subsystems `json:"subsystems"`
}

// Flags are the command line arguments of the server.
type Flags struct {
	Port       int
	ConfigPath string
}

// Default enables every subsystem.
func Default() Config {
	return Config{
		Subsystems: Subsystems{
			LoadAverage:  true,
			CPU:          true,
			DiskLoad:     true,
			DiskUsage:    true,
			NetworkStats: true,
		},
	}
}

// Load reads a JSON config. Subsystems missing from the file stay enabled,
// an empty path or an empty file means the defaults.
func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config: %w", err)
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return Config{}, fmt.Errorf("decode config %s: %w", path, err)
	}

	return cfg, nil
}

// ParseFlags parses the command line arguments (without the program name).
func ParseFlags(args []string, output io.Writer) (Flags, error) {
	var flags Flags

	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.SetOutput(output)
	fs.IntVar(&flags.Port, "port", defaultPort, "port of the gRPC server")
	fs.StringVar(&flags.ConfigPath, "config", "", "path to a JSON config with enabled subsystems (all enabled if empty)")

	if err := fs.Parse(args); err != nil {
		return Flags{}, err
	}

	if flags.Port < 1 || flags.Port > maxPort {
		return Flags{}, fmt.Errorf("invalid port %d: must be between 1 and %d", flags.Port, maxPort)
	}

	return flags, nil
}
