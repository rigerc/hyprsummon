package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootRequiresSubcommand(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{})

	err := root.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want missing command error")
	}
	if !strings.Contains(err.Error(), "a command is required: run, focus, or wizard") {
		t.Fatalf("Execute() error = %v, want missing command message", err)
	}
}

func TestOldRootStyleRejected(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"--class", "kitty", "--", "kitty"})

	err := root.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want unknown flag")
	}
	if !strings.Contains(err.Error(), "unknown flag: --class") {
		t.Fatalf("Execute() error = %v, want unknown --class flag", err)
	}
}

func TestRunAndFocusHelp(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "run help", args: []string{"run", "--help"}},
		{name: "focus help", args: []string{"focus", "--help"}},
		{name: "wizard help", args: []string{"wizard", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := newRootCmd()
			var buf bytes.Buffer
			root.SetOut(&buf)
			root.SetErr(&buf)
			root.SetArgs(tt.args)

			if err := root.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
		})
	}
}

func TestRootHelpShowsFlags(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "--class") {
		t.Fatalf("help output missing common flag: %q", out)
	}
	if !strings.Contains(out, "--background") {
		t.Fatalf("help output missing run-only flag: %q", out)
	}
	if !strings.Contains(out, "--scratch") {
		t.Fatalf("help output missing scratch flag: %q", out)
	}
}

func TestFlagHelpTopic(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"help", "flag", "--toggle-special"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "--toggle-special") {
		t.Fatalf("flag help output missing flag heading: %q", out)
	}
	if !strings.Contains(out, "Toggle visibility for the target special workspace") {
		t.Fatalf("flag help output missing summary: %q", out)
	}
}

func TestFocusRequiresClass(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"focus"})
	err := root.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want required class error")
	}
	if !strings.Contains(err.Error(), "required flag(s) \"class\" not set") {
		t.Fatalf("Execute() error = %v, want required class error", err)
	}
}

func TestRunRequiresLaunchCommand(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"run", "--class", "kitty"})
	err := root.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want minimum args error")
	}
	if !strings.Contains(err.Error(), "requires at least 1 arg(s), only received 0") {
		t.Fatalf("Execute() error = %v, want minimum args error", err)
	}
}
