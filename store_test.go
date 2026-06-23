package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadIPNotExist(t *testing.T) {
	_, err := readIP(filepath.Join(t.TempDir(), "ip"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}
}

func TestReadIPStripsWhitespace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ip")
	os.WriteFile(path, []byte("1.2.3.4\n"), 0644)
	ip, err := readIP(path)
	if err != nil {
		t.Fatal(err)
	}
	if ip != "1.2.3.4" {
		t.Errorf("got %q, want %q", ip, "1.2.3.4")
	}
}

func TestWriteIPCreatesDirectories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "dir", "ip")
	if err := writeIP(path, "5.6.7.8"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "5.6.7.8\n" {
		t.Errorf("got %q, want %q", string(data), "5.6.7.8\n")
	}
}

func TestWriteReadRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ip")
	if err := writeIP(path, "203.0.113.42"); err != nil {
		t.Fatal(err)
	}
	ip, err := readIP(path)
	if err != nil {
		t.Fatal(err)
	}
	if ip != "203.0.113.42" {
		t.Errorf("got %q, want %q", ip, "203.0.113.42")
	}
}
