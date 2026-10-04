package config

import (
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"
)

const portFlag = "-port"

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	return path
}

func TestLoad_EmptyPathMeansDefaults(t *testing.T) {
	got, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != Default() {
		t.Errorf("Load(\"\") = %+v, want %+v", got, Default())
	}
}

func TestLoad_EmptyFileMeansDefaults(t *testing.T) {
	got, err := Load(writeConfig(t, ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != Default() {
		t.Errorf("Load(empty file) = %+v, want %+v", got, Default())
	}
}

func TestLoad_DisablesOnlyListedSubsystems(t *testing.T) {
	got, err := Load(writeConfig(t, `{"subsystems": {"cpu": false, "diskLoad": false}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := Default()
	want.Subsystems.CPU = false
	want.Subsystems.DiskLoad = false

	if got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoad_Errors(t *testing.T) {
	cases := map[string]string{
		"unknown field": `{"subsystems": {"gpu": true}}`,
		"invalid json":  `{"subsystems": `,
		"wrong type":    `{"subsystems": {"cpu": "no"}}`,
	}

	for name, content := range cases {
		if _, err := Load(writeConfig(t, content)); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}

	if _, err := Load(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Error("missing file: expected error, got nil")
	}
}

func TestParseFlags(t *testing.T) {
	got, err := ParseFlags([]string{portFlag, "9000", "-config", "cfg.json"}, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := (Flags{Port: 9000, ConfigPath: "cfg.json"}); got != want {
		t.Errorf("ParseFlags() = %+v, want %+v", got, want)
	}
}

func TestParseFlags_Defaults(t *testing.T) {
	got, err := ParseFlags(nil, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := (Flags{Port: 8080}); got != want {
		t.Errorf("ParseFlags() = %+v, want %+v", got, want)
	}
}

func TestParseFlags_Errors(t *testing.T) {
	cases := map[string][]string{
		"port too small": {portFlag, "0"},
		"port too big":   {portFlag, "70000"},
		"not a number":   {portFlag, "abc"},
		"unknown flag":   {"-verbose"},
	}

	for name, args := range cases {
		if _, err := ParseFlags(args, io.Discard); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func TestParseFlags_Help(t *testing.T) {
	if _, err := ParseFlags([]string{"-h"}, io.Discard); !errors.Is(err, flag.ErrHelp) {
		t.Errorf("got %v, want %v", err, flag.ErrHelp)
	}
}

func TestLoad_ShippedConfigEnablesEverything(t *testing.T) {
	got, err := Load(filepath.Join("..", "..", "configs", "config.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != Default() {
		t.Errorf("configs/config.json = %+v, want %+v", got, Default())
	}
}
