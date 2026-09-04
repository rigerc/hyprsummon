package setup

import (
	"errors"
	"fmt"
	"path/filepath"
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
	BindKey           string
	BindDescription   string
	Executable        string
}

func NewState(format, bindKey, bindDescription, executable string) State {
	return State{
		Intent:            IntentRunOrRaise,
		CustomCommandKind: CommandRun,
		WorkspaceMode:     WorkspaceDefault,
		Preference:        PreferenceNone,
		FocusMode:         FocusModeNone,
		GenerateBind:      format == FormatBind,
		BindKey:           bindKey,
		BindDescription:   bindDescription,
		Executable:        executable,
	}
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

	command := s.BuildCommand()
	outputs := Outputs{Summary: s.Summary(), Command: command}
	if s.GenerateBind && format != FormatCommand {
		outputs.Bind = s.BuildBind(command)
	}
	return outputs, nil
}

func (s State) Validate(format string) error {
	if format != FormatCommand && format != FormatBind && format != FormatBoth {
		return fmt.Errorf("unknown output format %q", format)
	}
	if !filepath.IsAbs(s.Executable) {
		return errors.New("hyprsummon executable path must be absolute")
	}
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
	if s.GenerateBind && format != FormatCommand && strings.TrimSpace(s.BindKey) == "" {
		return errors.New("bind key is required")
	}
	if s.FocusMode != FocusModeNone && s.FocusMode != FocusModeFullscreen && s.FocusMode != FocusModeMaximize {
		return fmt.Errorf("unknown focus mode %q", s.FocusMode)
	}
	return nil
}

func (s State) BuildCommand() string {
	parts := []string{shellQuote(s.Executable), s.commandKind(), "--class", shellQuote(strings.TrimSpace(s.Class))}
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
	return strings.Join(parts, " ")
}

func (s State) BuildBind(command string) string {
	flags := "{ repeating = false"
	if description := strings.TrimSpace(s.BindDescription); description != "" {
		flags += ", description = " + luaQuote(description)
	}
	flags += " }"
	return fmt.Sprintf("hl.bind(%s, hl.dsp.exec_cmd(%s), %s)", luaQuote(strings.TrimSpace(s.BindKey)), luaQuote(command), flags)
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
	case IntentCustom:
		if s.CustomCommandKind == CommandFocus {
			return CommandFocus
		}
	}
	return CommandRun
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
	}
	return WorkspaceDefault
}

func (s State) usesSpecialWorkspace() bool {
	return s.Intent == IntentScratchApp || s.effectiveWorkspaceMode() == WorkspaceSpecial
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if !strings.ContainsAny(value, " \t\n'\"\\$`!&|;<>()[]{}*?~") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func luaQuote(value string) string {
	var output strings.Builder
	output.Grow(len(value) + 2)
	output.WriteByte('"')
	for _, b := range []byte(value) {
		switch b {
		case '\\':
			output.WriteString(`\\`)
		case '"':
			output.WriteString(`\"`)
		case '\b':
			output.WriteString(`\b`)
		case '\f':
			output.WriteString(`\f`)
		case '\n':
			output.WriteString(`\n`)
		case '\r':
			output.WriteString(`\r`)
		case '\t':
			output.WriteString(`\t`)
		default:
			if b < 0x20 || b == 0x7f {
				_, _ = fmt.Fprintf(&output, `\x%02X`, b)
			} else {
				output.WriteByte(b)
			}
		}
	}
	output.WriteByte('"')
	return output.String()
}
