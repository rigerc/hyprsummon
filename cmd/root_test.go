package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"hyprsummon/internal/hypr"
)

func TestResolveSocketPathUsesInstanceSignature(t *testing.T) {
	t.Parallel()

	runtimeDir := t.TempDir()
	got, err := resolveSocketPath(runtimeDir, "abc123")
	if err != nil {
		t.Fatalf("resolveSocketPath() error = %v", err)
	}

	want := filepath.Join(runtimeDir, "hypr", "abc123", ".socket.sock")
	if got != want {
		t.Fatalf("resolveSocketPath() = %q, want %q", got, want)
	}
}

func TestDiscoverSocketPathFindsOnlySocket(t *testing.T) {
	t.Parallel()

	runtimeDir := t.TempDir()
	want := makeSocketFile(t, runtimeDir, "instance-a", time.Now())

	got, err := discoverSocketPath(runtimeDir)
	if err != nil {
		t.Fatalf("discoverSocketPath() error = %v", err)
	}
	if got != want {
		t.Fatalf("discoverSocketPath() = %q, want %q", got, want)
	}
}

func TestDiscoverSocketPathPrefersNewestSocket(t *testing.T) {
	t.Parallel()

	runtimeDir := t.TempDir()
	oldTime := time.Now().Add(-1 * time.Hour)
	newTime := time.Now()
	_ = makeSocketFile(t, runtimeDir, "instance-old", oldTime)
	want := makeSocketFile(t, runtimeDir, "instance-new", newTime)

	got, err := discoverSocketPath(runtimeDir)
	if err != nil {
		t.Fatalf("discoverSocketPath() error = %v", err)
	}
	if got != want {
		t.Fatalf("discoverSocketPath() = %q, want %q", got, want)
	}
}

func TestDiscoverSocketPathErrorsWithoutSockets(t *testing.T) {
	t.Parallel()

	runtimeDir := t.TempDir()
	if _, err := discoverSocketPath(runtimeDir); err == nil {
		t.Fatal("discoverSocketPath() error = nil, want error")
	}
}

func TestApplyModesScratchForRun(t *testing.T) {
	t.Parallel()

	opts := hypr.Options{Scratch: true}
	applyModes("run", &opts)

	want := hypr.Options{
		PreferSpecial:          true,
		ToggleSpecial:          true,
		MoveToSpecial:          true,
		ShowSpecialAfterLaunch: true,
		Scratch:                true,
	}
	if !reflect.DeepEqual(opts, want) {
		t.Fatalf("applyModes() = %#v, want %#v", opts, want)
	}
}

func TestApplyModesScratchForFocus(t *testing.T) {
	t.Parallel()

	opts := hypr.Options{Scratch: true}
	applyModes("focus", &opts)

	want := hypr.Options{
		PreferSpecial: true,
		ToggleSpecial: true,
		MoveToSpecial: true,
		Scratch:       true,
	}
	if !reflect.DeepEqual(opts, want) {
		t.Fatalf("applyModes() = %#v, want %#v", opts, want)
	}
}

func makeSocketFile(t *testing.T, runtimeDir string, instance string, modTime time.Time) string {
	t.Helper()

	path := filepath.Join(runtimeDir, "hypr", instance, ".socket.sock")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("socket"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}

	return path
}
