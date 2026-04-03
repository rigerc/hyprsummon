package wizard

import (
	"strings"
	"testing"
)

func TestOutputsRunOrRaise(t *testing.T) {
	state := NewState(FormatBoth, "", "")
	state.Intent = IntentRunOrRaise
	state.Class = "kitty"
	state.LaunchCommand = "kitty"

	outputs, err := state.Outputs(FormatBoth)
	if err != nil {
		t.Fatalf("Outputs() error = %v", err)
	}
	want := "hyprsummon run --class kitty -- kitty"
	if outputs.Command != want {
		t.Fatalf("command = %q, want %q", outputs.Command, want)
	}
}

func TestOutputsFocusOnly(t *testing.T) {
	state := NewState(FormatBoth, "", "")
	state.Intent = IntentFocusOnly
	state.Class = "kitty"

	outputs, err := state.Outputs(FormatBoth)
	if err != nil {
		t.Fatalf("Outputs() error = %v", err)
	}
	want := "hyprsummon focus --class kitty"
	if outputs.Command != want {
		t.Fatalf("command = %q, want %q", outputs.Command, want)
	}
}

func TestOutputsScratchApp(t *testing.T) {
	state := NewState(FormatBoth, "", "")
	state.Intent = IntentScratchApp
	state.Class = "spotify"
	state.SpecialWorkspace = "music"
	state.UseScratch = true
	state.LaunchCommand = "spotify-launcher"

	outputs, err := state.Outputs(FormatBoth)
	if err != nil {
		t.Fatalf("Outputs() error = %v", err)
	}
	want := "hyprsummon run --class spotify --special-workspace music --scratch -- spotify-launcher"
	if outputs.Command != want {
		t.Fatalf("command = %q, want %q", outputs.Command, want)
	}
}

func TestOutputsBindd(t *testing.T) {
	state := NewState(FormatBind, "$mainMod, M", "Start/toggle Spotify")
	state.Intent = IntentScratchApp
	state.Class = "spotify"
	state.SpecialWorkspace = "music"
	state.UseScratch = true
	state.LaunchCommand = "spotify-launcher"
	state.BindStyle = BindStyleBindd

	outputs, err := state.Outputs(FormatBind)
	if err != nil {
		t.Fatalf("Outputs() error = %v", err)
	}
	if !strings.Contains(outputs.Bind, "bindd = $mainMod, M, Start/toggle Spotify, exec, ") {
		t.Fatalf("bind = %q, want bindd output", outputs.Bind)
	}
}

func TestOutputsBindRequiresKey(t *testing.T) {
	state := NewState(FormatBind, "", "")
	state.Intent = IntentRunOrRaise
	state.Class = "kitty"
	state.LaunchCommand = "kitty"
	state.GenerateBind = true

	_, err := state.Outputs(FormatBind)
	if err == nil || err.Error() != "bind key is required" {
		t.Fatalf("Outputs() error = %v, want bind key error", err)
	}
}

func TestOutputsRequireLaunchForRun(t *testing.T) {
	state := NewState(FormatBoth, "", "")
	state.Intent = IntentRunOrRaise
	state.Class = "kitty"

	_, err := state.Outputs(FormatBoth)
	if err == nil || err.Error() != "launch command is required for run" {
		t.Fatalf("Outputs() error = %v, want launch command error", err)
	}
}

func TestOutputsRejectFocusLaunch(t *testing.T) {
	state := NewState(FormatBoth, "", "")
	state.Intent = IntentFocusOnly
	state.Class = "kitty"
	state.LaunchCommand = "kitty"

	_, err := state.Outputs(FormatBoth)
	if err == nil || err.Error() != "focus must not include a launch command" {
		t.Fatalf("Outputs() error = %v, want focus launch error", err)
	}
}
