package setup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	hyprland "github.com/thiagokokada/hyprland-go"
)

func TestDiscoverAppsGroupsMappedClientsAndSanitizesLabels(t *testing.T) {
	t.Parallel()

	clients := []hyprland.Client{
		{Address: "0x2", Mapped: true, Class: "zen", InitialClass: "zen-alpha", Title: "Second", Workspace: hyprland.WorkspaceType{Name: "2"}},
		{Address: "0x1", Mapped: true, Class: "code", InitialClass: "code", Title: "Editor", Workspace: hyprland.WorkspaceType{Name: "1"}},
		{Address: "0x3", Mapped: true, Class: "zen", InitialClass: "zen-alpha", Title: "Browser\x1b[31m\n", Workspace: hyprland.WorkspaceType{Name: "1"}},
		{Address: "0x4", Mapped: false, Class: "ignored"},
		{Address: "0x5", Mapped: true, Class: ""},
	}

	got := DiscoverApps(clients, "0x3")
	want := []AppChoice{
		{Class: "code", InitialClass: "code", Title: "Editor", Workspace: "1", Count: 1},
		{Class: "zen", InitialClass: "zen-alpha", Title: "Browser\x1b[31m\n", Workspace: "1", Count: 2, Active: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DiscoverApps() = %#v, want %#v", got, want)
	}

	for _, unsafe := range []string{"\x1b", "\n", "\t"} {
		if strings.Contains(got[1].Label(), unsafe) {
			t.Fatalf("Label() = %q, contains unsafe text %q", got[1].Label(), unsafe)
		}
	}
}

func TestOutputsGenerateAbsoluteCommandAndLuaBind(t *testing.T) {
	t.Parallel()

	state := NewState(FormatBoth, `SUPER + SHIFT + "M"`, "Raise 音楽", "/opt/Hypr Summon/bin/hyprsummon")
	state.Intent = IntentScratchApp
	state.Class = "weird 'class'"
	state.InitialClass = `initial\\class`
	state.SpecialWorkspace = "music"
	state.UseScratch = true
	state.LaunchCommand = `printf '%s\n' "$HOME; literal"`
	state.GenerateBind = true

	outputs, err := state.Outputs(FormatBoth)
	if err != nil {
		t.Fatalf("Outputs() error = %v", err)
	}

	for _, want := range []string{
		`'/opt/Hypr Summon/bin/hyprsummon' run`,
		`--class 'weird '"'"'class'"'"''`,
		`--initial-class 'initial\\class'`,
		`--scratch -- printf '%s\n' "$HOME; literal"`,
	} {
		if !strings.Contains(outputs.Command, want) {
			t.Fatalf("Command = %q, missing %q", outputs.Command, want)
		}
	}
	for _, want := range []string{
		`hl.bind("SUPER + SHIFT + \"M\"", hl.dsp.exec_cmd(`,
		`{ repeating = false, description = "Raise 音楽" })`,
	} {
		if !strings.Contains(outputs.Bind, want) {
			t.Fatalf("Bind = %q, missing %q", outputs.Bind, want)
		}
	}
	if strings.Contains(outputs.Bind, "bindd =") {
		t.Fatalf("Bind = %q, contains legacy bindd syntax", outputs.Bind)
	}
}

func TestOutputsFocusOnly(t *testing.T) {
	t.Parallel()

	state := NewState(FormatCommand, "", "", "/usr/bin/hyprsummon")
	state.Intent = IntentFocusOnly
	state.Class = "kitty"

	outputs, err := state.Outputs(FormatCommand)
	if err != nil {
		t.Fatalf("Outputs() error = %v", err)
	}
	if outputs.Command != "/usr/bin/hyprsummon focus --class kitty" {
		t.Fatalf("Command = %q", outputs.Command)
	}
}

func TestOutputsCommandFormatNeverGeneratesBind(t *testing.T) {
	t.Parallel()

	state := NewState(FormatCommand, "SUPER + K", "Ignored", "/usr/bin/hyprsummon")
	state.Intent = IntentFocusOnly
	state.Class = "kitty"
	state.GenerateBind = true

	outputs, err := state.Outputs(FormatCommand)
	if err != nil {
		t.Fatalf("Outputs() error = %v", err)
	}
	if outputs.Bind != "" {
		t.Fatalf("Bind = %q, want no bind for command format", outputs.Bind)
	}
}

type fakeCommandRunner struct {
	args   []string
	output []byte
	err    error
}

func (r *fakeCommandRunner) Run(_ context.Context, path string, args ...string) ([]byte, error) {
	r.args = append([]string{path}, args...)
	return r.output, r.err
}

func TestLuaValidatorParsesWithoutExecutingChunk(t *testing.T) {
	t.Parallel()

	runner := &fakeCommandRunner{output: []byte("ok\n")}
	validator := LuaValidator{HyprctlPath: "/usr/bin/hyprctl", Runner: runner}
	snippet := `hl.bind("SUPER + M", hl.dsp.exec_cmd("hyprsummon"))`

	if err := validator.Validate(context.Background(), snippet); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if len(runner.args) != 3 || runner.args[0] != "/usr/bin/hyprctl" || runner.args[1] != "repl" {
		t.Fatalf("runner args = %#v", runner.args)
	}
	code := runner.args[2]
	if !strings.Contains(code, "load(") || !strings.Contains(code, `return err or "ok"`) {
		t.Fatalf("validation code = %q", code)
	}
	if strings.Contains(code, "load(") && strings.Contains(code, ")()") {
		t.Fatalf("validation code executes loaded chunk: %q", code)
	}
}

func TestLuaValidatorReportsParserRejection(t *testing.T) {
	t.Parallel()

	runner := &fakeCommandRunner{output: []byte("unfinished string near <eof>\n")}
	err := (LuaValidator{HyprctlPath: "hyprctl", Runner: runner}).Validate(context.Background(), "broken")
	if err == nil || !strings.Contains(err.Error(), "unfinished string") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestWriteOutputRefusesOverwriteUnlessForced(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "hyprsummon.lua")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteOutput(path, "replace", false); err == nil {
		t.Fatal("WriteOutput() error = nil")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "keep" {
		t.Fatalf("file = %q, want preserved", data)
	}
	if err := WriteOutput(path, "replace", true); err != nil {
		t.Fatalf("WriteOutput(force) error = %v", err)
	}
	data, _ = os.ReadFile(path)
	if string(data) != "replace" {
		t.Fatalf("file = %q, want replacement", data)
	}
}

type fakeClient struct {
	clients     []hyprland.Client
	active      hyprland.Window
	clientCalls int
	versionTag  string
	clientsErr  error
}

func (c *fakeClient) Clients() ([]hyprland.Client, error) {
	c.clientCalls++
	return c.clients, c.clientsErr
}

func (c *fakeClient) ActiveWindow() (hyprland.Window, error) { return c.active, nil }

func (c *fakeClient) Version() (hyprland.Version, error) {
	tag := c.versionTag
	if tag == "" {
		tag = "v0.56.2"
	}
	return hyprland.Version{Tag: tag}, nil
}

type fakePrompter struct {
	choices []string
	calls   int
	config  func(*State)
	err     error
}

func (p *fakePrompter) ChooseApp(_ context.Context, _ []AppChoice) (string, error) {
	if p.err != nil {
		return "", p.err
	}
	choice := p.choices[p.calls]
	p.calls++
	return choice, nil
}

func (p *fakePrompter) Configure(_ context.Context, state *State) error {
	if p.err != nil {
		return p.err
	}
	p.config(state)
	return nil
}

type fakeValidator struct {
	snippet string
}

func (v *fakeValidator) Validate(_ context.Context, snippet string) error {
	v.snippet = snippet
	return nil
}

func TestRunRefreshesAndGeneratesValidatedOutput(t *testing.T) {
	t.Parallel()

	client := &fakeClient{
		clients: []hyprland.Client{{Address: "0x1", Mapped: true, Class: "kitty", InitialClass: "kitty", Title: "Shell"}},
		active:  hyprland.Window{Client: hyprland.Client{Address: "0x1"}},
	}
	prompter := &fakePrompter{
		choices: []string{ChoiceRefresh, "kitty"},
		config: func(state *State) {
			if state.Class != "kitty" || state.Title != "" || state.InitialClass != "" {
				t.Fatalf("discovery defaults = %#v; want class-only matching", state)
			}
			state.Intent = IntentRunOrRaise
			state.LaunchCommand = "kitty"
			state.GenerateBind = true
			state.BindKey = "SUPER + K"
		},
	}
	validator := &fakeValidator{}
	path := filepath.Join(t.TempDir(), "hyprsummon.lua")

	outputs, err := Run(context.Background(), Config{Format: FormatBoth, OutputPath: path}, Runtime{
		Client:     client,
		Prompter:   prompter,
		Validator:  validator,
		Executable: "/usr/bin/hyprsummon",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if client.clientCalls != 2 {
		t.Fatalf("Clients() calls = %d, want 2", client.clientCalls)
	}
	if outputs.Bind == "" || validator.snippet != outputs.Bind {
		t.Fatalf("outputs = %#v, validated = %q", outputs, validator.snippet)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil || string(data) != outputs.Bind+"\n" {
		t.Fatalf("output file = %q, error = %v", data, readErr)
	}
}

func TestHuhPrompterAccessibleDiscoveryHasNoANSI(t *testing.T) {
	t.Parallel()

	var output strings.Builder
	prompter := NewHuhPrompter(Config{
		Accessible:   true,
		Input:        strings.NewReader("1\n"),
		PromptOutput: &output,
	})
	choice, err := prompter.ChooseApp(context.Background(), []AppChoice{{Class: "kitty", Title: "Shell", Count: 1}})
	if err != nil {
		t.Fatalf("ChooseApp() error = %v", err)
	}
	if choice != "kitty" {
		t.Fatalf("choice = %q, want kitty", choice)
	}
	if strings.Contains(output.String(), "\x1b") {
		t.Fatalf("accessible output contains ANSI: %q", output.String())
	}
}

func TestRunAccessibleFlowUsesOneInputStream(t *testing.T) {
	t.Parallel()

	input := strings.NewReader("1\n1\nkitty\n\n\nkitty\nSUPER + K\nSummon Kitty\n")
	var prompts strings.Builder
	cfg := Config{
		Format:       FormatBind,
		BindKey:      "SUPER + K",
		BindDesc:     "Summon Kitty",
		Accessible:   true,
		Input:        input,
		PromptOutput: &prompts,
	}
	validator := &fakeValidator{}
	client := &fakeClient{
		clients: []hyprland.Client{{Address: "0x1", Mapped: true, Class: "kitty", Title: "Shell"}},
		active:  hyprland.Window{Client: hyprland.Client{Address: "0x1"}},
	}

	outputs, err := Run(context.Background(), cfg, Runtime{
		Client:     client,
		Prompter:   NewHuhPrompter(cfg),
		Validator:  validator,
		Executable: "/usr/bin/hyprsummon",
	})
	if err != nil {
		t.Fatalf("Run() error = %v; prompts = %q", err, prompts.String())
	}
	if outputs.Bind == "" || validator.snippet == "" {
		t.Fatalf("outputs = %#v, validated = %q", outputs, validator.snippet)
	}
	if strings.Contains(prompts.String(), "\x1b") {
		t.Fatalf("accessible prompts contain ANSI: %q", prompts.String())
	}
}

func TestRunCancellationDoesNotWrite(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing.lua")
	_, err := Run(context.Background(), Config{Format: FormatBoth, OutputPath: path}, Runtime{
		Client:     &fakeClient{},
		Prompter:   &fakePrompter{err: ErrCanceled},
		Validator:  &fakeValidator{},
		Executable: "/usr/bin/hyprsummon",
	})
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("Run() error = %v", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("output exists after cancellation: %v", statErr)
	}
}

func TestRunManualClassFromEmptyDiscovery(t *testing.T) {
	t.Parallel()

	prompter := &fakePrompter{
		choices: []string{ChoiceManual},
		config: func(state *State) {
			state.Intent = IntentFocusOnly
			state.Class = "org.example.App"
		},
	}
	outputs, err := Run(context.Background(), Config{Format: FormatCommand}, Runtime{
		Client:     &fakeClient{},
		Prompter:   prompter,
		Executable: "/usr/bin/hyprsummon",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if outputs.Command != "/usr/bin/hyprsummon focus --class org.example.App" {
		t.Fatalf("Command = %q", outputs.Command)
	}
}

func TestRunRejectsLegacyHyprlandBeforePromptingOrWriting(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing.lua")
	_, err := Run(context.Background(), Config{Format: FormatBind, OutputPath: path}, Runtime{
		Client:     &fakeClient{versionTag: "v0.54.1"},
		Prompter:   &fakePrompter{},
		Validator:  &fakeValidator{},
		Executable: "/usr/bin/hyprsummon",
	})
	if err == nil || !strings.Contains(err.Error(), "0.55 or newer") {
		t.Fatalf("Run() error = %v", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("output exists after version rejection: %v", statErr)
	}
}

func TestRunDiscoveryErrorDoesNotWrite(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing.lua")
	_, err := Run(context.Background(), Config{Format: FormatBind, OutputPath: path}, Runtime{
		Client:     &fakeClient{clientsErr: errors.New("socket closed")},
		Prompter:   &fakePrompter{},
		Validator:  &fakeValidator{},
		Executable: "/usr/bin/hyprsummon",
	})
	if err == nil || !strings.Contains(err.Error(), "discover Hyprland clients") {
		t.Fatalf("Run() error = %v", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("output exists after discovery error: %v", statErr)
	}
}
