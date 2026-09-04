package setup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	hyprland "github.com/thiagokokada/hyprland-go"
)

const (
	ChoiceManual  = "\x00manual"
	ChoiceRefresh = "\x00refresh"
)

var ErrCanceled = errors.New("setup canceled")

type Config struct {
	Format       string
	BindKey      string
	BindDesc     string
	Advanced     bool
	Accessible   bool
	NoColor      bool
	Theme        string
	OutputPath   string
	Force        bool
	Input        io.Reader
	PromptOutput io.Writer
}

type Client interface {
	Clients() ([]hyprland.Client, error)
	ActiveWindow() (hyprland.Window, error)
	Version() (hyprland.Version, error)
}

type Prompter interface {
	ChooseApp(context.Context, []AppChoice) (string, error)
	Configure(context.Context, *State) error
}

type Validator interface {
	Validate(context.Context, string) error
}

type Runtime struct {
	Client     Client
	Prompter   Prompter
	Validator  Validator
	Executable string
}

func Run(ctx context.Context, cfg Config, runtime Runtime) (Outputs, error) {
	if runtime.Client == nil || runtime.Prompter == nil {
		return Outputs{}, errors.New("setup runtime is incomplete")
	}
	version, err := runtime.Client.Version()
	if err != nil {
		return Outputs{}, fmt.Errorf("query Hyprland version: %w", err)
	}
	if !supportsLuaConfig(version.Tag) {
		return Outputs{}, fmt.Errorf("Hyprland %s is unsupported: Lua configuration requires Hyprland 0.55 or newer", version.Tag)
	}

	state := NewState(cfg.Format, cfg.BindKey, cfg.BindDesc, runtime.Executable)
	for {
		clients, err := runtime.Client.Clients()
		if err != nil {
			return Outputs{}, fmt.Errorf("discover Hyprland clients: %w", err)
		}
		activeWindow, err := runtime.Client.ActiveWindow()
		if err != nil {
			return Outputs{}, fmt.Errorf("query active Hyprland window: %w", err)
		}
		choices := DiscoverApps(clients, activeWindow.Address)
		choice, err := runtime.Prompter.ChooseApp(ctx, choices)
		if err != nil {
			return Outputs{}, err
		}
		if choice == ChoiceRefresh {
			continue
		}
		if choice != ChoiceManual {
			for _, candidate := range choices {
				if candidate.Class != choice {
					continue
				}
				state.Class = candidate.Class
				break
			}
		}
		break
	}

	if err := runtime.Prompter.Configure(ctx, &state); err != nil {
		return Outputs{}, err
	}
	outputs, err := state.Outputs(cfg.Format)
	if err != nil {
		return Outputs{}, err
	}
	if outputs.Bind != "" {
		if runtime.Validator == nil {
			return Outputs{}, errors.New("Lua validator is required for bind output")
		}
		if err := runtime.Validator.Validate(ctx, outputs.Bind); err != nil {
			return Outputs{}, err
		}
	}
	if cfg.OutputPath != "" {
		content := outputs.Command
		if outputs.Bind != "" {
			content = outputs.Bind
		}
		if err := WriteOutput(cfg.OutputPath, content+"\n", cfg.Force); err != nil {
			return Outputs{}, err
		}
	}
	return outputs, nil
}

func supportsLuaConfig(tag string) bool {
	clean := strings.TrimPrefix(strings.TrimSpace(tag), "v")
	if clean == "" {
		return true
	}
	parts := strings.Split(clean, ".")
	if len(parts) < 2 {
		return true
	}
	major, majorErr := strconv.Atoi(parts[0])
	minor, minorErr := strconv.Atoi(parts[1])
	if majorErr != nil || minorErr != nil {
		return true
	}
	return major > 0 || minor >= 55
}

type CommandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, path string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, path, args...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err == nil {
		return output, nil
	}
	detail := strings.TrimSpace(stderr.String())
	if detail == "" {
		return nil, fmt.Errorf("run %s: %w", path, err)
	}
	return nil, fmt.Errorf("run %s: %w: %s", path, err, detail)
}

type LuaValidator struct {
	HyprctlPath string
	Runner      CommandRunner
}

func (v LuaValidator) Validate(ctx context.Context, snippet string) error {
	path := v.HyprctlPath
	if path == "" {
		path = "hyprctl"
	}
	runner := v.Runner
	if runner == nil {
		runner = ExecRunner{}
	}
	code := "local _, err = load(" + luaQuote(snippet) + "); return err or \"ok\""
	output, err := runner.Run(ctx, path, "repl", code)
	if err != nil {
		return fmt.Errorf("validate generated Hyprland Lua: %w", err)
	}
	result := strings.TrimSpace(string(output))
	if result != "ok" {
		return fmt.Errorf("validate generated Hyprland Lua: %s", result)
	}
	return nil
}

func WriteOutput(path, content string, force bool) error {
	flags := os.O_WRONLY | os.O_CREATE
	if force {
		if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
			return fmt.Errorf("refuse to overwrite non-regular output %q", path)
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect output %q: %w", path, err)
		}
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	file, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("output %q already exists; use --force to overwrite it", path)
		}
		return fmt.Errorf("open output %q: %w", path, err)
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		return fmt.Errorf("write output %q: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close output %q: %w", path, err)
	}
	return nil
}
