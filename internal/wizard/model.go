package wizard

import (
	"errors"
	"fmt"
	"strings"
)

const (
	IntentRunOrRaise = "run_or_raise"
	IntentFocusOnly  = "focus_only"
	IntentBringHere  = "bring_here"
	IntentScratchApp = "scratch_app"
	IntentCustom     = "custom"
)

const (
	CommandRun   = "run"
	CommandFocus = "focus"
)

const (
	WorkspaceDefault     = "default"
	WorkspaceCurrentOnly = "current_only"
	WorkspacePull        = "pull"
	WorkspaceSpecial     = "special"
)

const (
	PreferenceNone           = "none"
	PreferenceFloating       = "floating"
	PreferenceTiled          = "tiled"
	PreferencePreferSpecial  = "prefer_special"
	PreferenceExcludeSpecial = "exclude_special"
)

const (
	FocusModeNone       = "none"
	FocusModeFullscreen = "fullscreen"
	FocusModeMaximize   = "maximize"
)

const (
	FormatCommand = "command"
	FormatBind    = "bind"
	FormatBoth    = "both"
)

const (
	BindStyleBind  = "bind"
	BindStyleBindd = "bindd"
)

type State struct {
	Intent            string
	Class             string
	Title             string
	InitialClass      string
	LaunchCommand     string
	CustomCommandKind string
	WorkspaceMode     string
	SpecialWorkspace  string
	UseScratch        bool
	Cycle             bool
	Preference        string
	FocusMode         string
	Notify            bool
	Verbose           bool
	Debug             bool
	GenerateBind      bool
	BindStyle         string
	BindKey           string
	BindDescription   string
}

func NewState(format string, bindKey string, bindDesc string) State {
	state := State{
		Intent:            IntentRunOrRaise,
		CustomCommandKind: CommandRun,
		WorkspaceMode:     WorkspaceDefault,
		Preference:        PreferenceNone,
		FocusMode:         FocusModeNone,
		BindStyle:         BindStyleBindd,
		BindKey:           bindKey,
		BindDescription:   bindDesc,
	}
	if format == FormatBind {
		state.GenerateBind = true
	}
	return state
}

type Outputs struct {
	Summary string
	Command string
	Bind    string
}

func (s State) Outputs(format string) (Outputs, error) {
	if err := s.Validate(format); err != nil {
		return Outputs{}, err
	}

	command, err := s.BuildCommand()
	if err != nil {
		return Outputs{}, err
	}

	outputs := Outputs{
		Summary: s.Summary(),
		Command: command,
	}
	if s.GenerateBind {
		outputs.Bind, err = s.BuildBind(command)
		if err != nil {
			return Outputs{}, err
		}
	}
	return outputs, nil
}

func (s State) Validate(format string) error {
	if strings.TrimSpace(s.Class) == "" {
		return errors.New("class is required")
	}
	if kind := s.commandKind(); kind == CommandRun && strings.TrimSpace(s.LaunchCommand) == "" {
		return errors.New("launch command is required for run")
	}
	if kind := s.commandKind(); kind == CommandFocus && strings.TrimSpace(s.LaunchCommand) != "" {
		return errors.New("focus must not include a launch command")
	}
	if s.usesSpecialWorkspace() && strings.TrimSpace(s.SpecialWorkspace) == "" {
		return errors.New("special workspace is required")
	}
	if s.UseScratch && !s.usesSpecialWorkspace() {
		return errors.New("scratch mode requires a special workspace")
	}
	if format == FormatBind && !s.GenerateBind {
		return errors.New("bind output requires bind generation")
	}
	if s.GenerateBind && strings.TrimSpace(s.BindKey) == "" {
		return errors.New("bind key is required")
	}
	if s.GenerateBind && s.BindStyle == BindStyleBindd && strings.TrimSpace(s.BindDescription) == "" {
		return errors.New("bind description is required for bindd")
	}
	if s.FocusMode != FocusModeNone && s.FocusMode != FocusModeFullscreen && s.FocusMode != FocusModeMaximize {
		return fmt.Errorf("unknown focus mode %q", s.FocusMode)
	}
	return nil
}

func (s State) BuildCommand() (string, error) {
	parts := []string{"hyprsummon", s.commandKind()}
	parts = append(parts, "--class", shellQuote(strings.TrimSpace(s.Class)))

	if title := strings.TrimSpace(s.Title); title != "" {
		parts = append(parts, "--title", shellQuote(title))
	}
	if initialClass := strings.TrimSpace(s.InitialClass); initialClass != "" {
		parts = append(parts, "--initial-class", shellQuote(initialClass))
	}

	if s.usesSpecialWorkspace() {
		parts = append(parts, "--special-workspace", shellQuote(strings.TrimSpace(s.SpecialWorkspace)))
	}

	if s.UseScratch {
		parts = append(parts, "--scratch")
	} else {
		switch s.effectiveWorkspaceMode() {
		case WorkspaceCurrentOnly:
			parts = append(parts, "--current-workspace-only")
		case WorkspacePull:
			parts = append(parts, "--pull")
		case WorkspaceSpecial:
			if s.commandKind() == CommandFocus {
				parts = append(parts, "--toggle-special")
			}
		}
	}

	switch s.Preference {
	case PreferenceFloating:
		parts = append(parts, "--prefer-floating")
	case PreferenceTiled:
		parts = append(parts, "--prefer-tiled")
	case PreferencePreferSpecial:
		parts = append(parts, "--prefer-special")
	case PreferenceExcludeSpecial:
		parts = append(parts, "--exclude-special")
	}

	if s.Cycle {
		parts = append(parts, "--cycle")
	}
	switch s.FocusMode {
	case FocusModeFullscreen:
		parts = append(parts, "--fullscreen")
	case FocusModeMaximize:
		parts = append(parts, "--maximize")
	}
	if s.Notify {
		parts = append(parts, "--notify")
	}
	if s.Verbose {
		parts = append(parts, "--verbose")
	}
	if s.Debug {
		parts = append(parts, "--debug")
	}

	if s.commandKind() == CommandRun {
		parts = append(parts, "--", strings.TrimSpace(s.LaunchCommand))
	}

	return strings.Join(parts, " "), nil
}

func (s State) BuildBind(command string) (string, error) {
	key := strings.TrimSpace(s.BindKey)
	if key == "" {
		return "", errors.New("bind key is required")
	}
	switch s.BindStyle {
	case "", BindStyleBind:
		return fmt.Sprintf("bind = %s, exec, %s", key, command), nil
	case BindStyleBindd:
		desc := strings.TrimSpace(s.BindDescription)
		if desc == "" {
			return "", errors.New("bind description is required for bindd")
		}
		return fmt.Sprintf("bindd = %s, %s, exec, %s", key, desc, command), nil
	default:
		return "", fmt.Errorf("unknown bind style %q", s.BindStyle)
	}
}

func (s State) Summary() string {
	switch s.Intent {
	case IntentFocusOnly:
		return fmt.Sprintf("Focus an existing %s window only.", s.Class)
	case IntentBringHere:
		return fmt.Sprintf("Run or raise %s and pull an existing match to the current workspace.", s.Class)
	case IntentScratchApp:
		return fmt.Sprintf("Run or raise %s as a scratch app on special:%s.", s.Class, s.SpecialWorkspace)
	case IntentCustom:
		if s.UseScratch {
			return fmt.Sprintf("Custom setup: keep %s on special:%s with scratch semantics.", s.Class, s.SpecialWorkspace)
		}
		return fmt.Sprintf("Custom setup for %s using %s behavior.", s.Class, strings.ReplaceAll(s.effectiveWorkspaceMode(), "_", " "))
	default:
		return fmt.Sprintf("Run or raise %s.", s.Class)
	}
}

func (s State) commandKind() string {
	switch s.Intent {
	case IntentFocusOnly:
		return CommandFocus
	case IntentBringHere, IntentRunOrRaise, IntentScratchApp:
		return CommandRun
	case IntentCustom:
		if s.CustomCommandKind == CommandFocus {
			return CommandFocus
		}
		return CommandRun
	default:
		return CommandRun
	}
}

func (s State) effectiveWorkspaceMode() string {
	switch s.Intent {
	case IntentBringHere:
		return WorkspacePull
	case IntentScratchApp:
		return WorkspaceSpecial
	case IntentCustom:
		if s.WorkspaceMode != "" {
			return s.WorkspaceMode
		}
		return WorkspaceDefault
	default:
		return WorkspaceDefault
	}
}

func (s State) usesSpecialWorkspace() bool {
	return s.Intent == IntentScratchApp || s.effectiveWorkspaceMode() == WorkspaceSpecial
}

func shellQuote(arg string) string {
	if arg == "" {
		return "''"
	}
	if !strings.ContainsAny(arg, " \t\n'\"\\$`!&|;<>()[]{}*?~") {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", `'"'"'`) + "'"
}
