package client

import (
	"errors"
	"flag"
	"io"
	"reflect"
	"testing"
)

const sectionsFlag = "-sections"

func TestParseFlags_Defaults(t *testing.T) {
	got, err := ParseFlags(nil, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := Options{Addr: "localhost:8080", N: 5, M: 15, Sections: allSectionsSet()}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseFlags() = %+v, want %+v", got, want)
	}
}

func TestParseFlags_Custom(t *testing.T) {
	args := []string{"-addr", "host:9000", "-n", "2", "-m", "10", sectionsFlag, "cpu, NetStats"}

	got, err := ParseFlags(args, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := Options{
		Addr:     "host:9000",
		N:        2,
		M:        10,
		Sections: Sections{SectionCPU: true, SectionNetStats: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseFlags() = %+v, want %+v", got, want)
	}
}

func TestParseFlags_AllAndEmptySectionsMeanEverything(t *testing.T) {
	for _, value := range []string{"all", "", " , ", "cpu,all"} {
		got, err := ParseFlags([]string{sectionsFlag, value}, io.Discard)
		if err != nil {
			t.Fatalf("sections %q: unexpected error: %v", value, err)
		}

		if !reflect.DeepEqual(got.Sections, allSectionsSet()) {
			t.Errorf("sections %q = %v, want all", value, got.Sections)
		}
	}
}

func TestParseFlags_Errors(t *testing.T) {
	cases := map[string][]string{
		"unknown section": {sectionsFlag, "cpu,gpu"},
		"unknown flag":    {"-verbose"},
		"bad number":      {"-n", "abc"},
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
