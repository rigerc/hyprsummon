package wizard

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	huh "charm.land/huh/v2"
)

type Config struct {
	Format     string
	BindKey    string
	BindDesc   string
	Advanced   bool
	Accessible bool
	Theme      string
	Output     io.Writer
}

func Run(cfg Config) (Outputs, error) {
	state := NewState(cfg.Format, cfg.BindKey, cfg.BindDesc)
	form := BuildForm(cfg, &state)
	if err := form.Run(); err != nil {
		return Outputs{}, err
	}
	return state.Outputs(cfg.Format)
}

func BuildForm(cfg Config, state *State) *huh.Form {
	intentGroup := huh.NewGroup(
		huh.NewSelect[string]().
			Title("What do you want to set up?").
			Description("Start from a preset and let the wizard generate the command.").
			Options(
				huh.NewOption("Run or raise", IntentRunOrRaise),
				huh.NewOption("Focus existing only", IntentFocusOnly),
				huh.NewOption("Bring existing window here", IntentBringHere),
				huh.NewOption("Scratch app on a special workspace", IntentScratchApp),
				huh.NewOption("Custom", IntentCustom),
			).
			Value(&state.Intent),
	).Title("Intent")

	targetGroup := huh.NewGroup(
		huh.NewInput().
			Title("Window class").
			Description("Exact Hyprland class to match.").
			Value(&state.Class).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return errors.New("class is required")
				}
				return nil
			}),
		huh.NewInput().
			Title("Window title").
			Description("Optional exact title match. Leave blank to ignore.").
			Value(&state.Title),
		huh.NewInput().
			Title("Initial class").
			Description("Optional exact initialClass match. Leave blank to ignore.").
			Value(&state.InitialClass),
	).Title("Target")

	launchGroup := huh.NewGroup(
		huh.NewInput().
			Title("Launch command").
			Description("Command to run after `--` when no match exists.").
			Value(&state.LaunchCommand).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return errors.New("launch command is required for run")
				}
				return nil
			}),
	).Title("Launch").WithHideFunc(func() bool {
		return commandKindForForm(state) != CommandRun
	})

	customGroup := huh.NewGroup(
		huh.NewSelect[string]().
			Title("Base command").
			Options(
				huh.NewOption("run", CommandRun),
				huh.NewOption("focus", CommandFocus),
			).
			Value(&state.CustomCommandKind),
		huh.NewSelect[string]().
			Title("Workspace behavior").
			Options(
				huh.NewOption("Default", WorkspaceDefault),
				huh.NewOption("Current workspace only", WorkspaceCurrentOnly),
				huh.NewOption("Pull existing window here", WorkspacePull),
				huh.NewOption("Named special workspace", WorkspaceSpecial),
			).
			Value(&state.WorkspaceMode),
	).Title("Custom").WithHideFunc(func() bool {
		return state.Intent != IntentCustom
	})

	specialGroup := huh.NewGroup(
		huh.NewInput().
			Title("Special workspace name").
			Description("Use the bare name, not `special:NAME`.").
			Value(&state.SpecialWorkspace).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return errors.New("special workspace is required")
				}
				return nil
			}),
		huh.NewConfirm().
			Title("Use scratch mode?").
			Description("Recommended for app toggles like music players or scratch terminals.").
			Value(&state.UseScratch),
	).Title("Special Workspace").WithHideFunc(func() bool {
		return !needsSpecialWorkspace(state)
	})

	advancedGroup := huh.NewGroup(
		huh.NewConfirm().
			Title("Cycle through multiple matches?").
			Value(&state.Cycle),
		huh.NewSelect[string]().
			Title("Match preference").
			Options(
				huh.NewOption("None", PreferenceNone),
				huh.NewOption("Prefer floating", PreferenceFloating),
				huh.NewOption("Prefer tiled", PreferenceTiled),
				huh.NewOption("Prefer special", PreferencePreferSpecial),
				huh.NewOption("Exclude special", PreferenceExcludeSpecial),
			).
			Value(&state.Preference),
		huh.NewSelect[string]().
			Title("Post-focus mode").
			Options(
				huh.NewOption("None", FocusModeNone),
				huh.NewOption("Fullscreen", FocusModeFullscreen),
				huh.NewOption("Maximize", FocusModeMaximize),
			).
			Value(&state.FocusMode),
		huh.NewConfirm().Title("Notify?").Value(&state.Notify),
		huh.NewConfirm().Title("Verbose stderr diagnostics?").Value(&state.Verbose),
		huh.NewConfirm().Title("Desktop debug notifications?").Value(&state.Debug),
	).Title("Advanced").WithHideFunc(func() bool {
		return !(cfg.Advanced || state.Intent == IntentCustom)
	})

	bindAskGroup := huh.NewGroup(
		huh.NewConfirm().
			Title("Generate a Hyprland bind snippet?").
			Value(&state.GenerateBind),
	).Title("Bind").WithHideFunc(func() bool {
		return cfg.Format == FormatBind
	})

	bindGroup := huh.NewGroup(
		huh.NewSelect[string]().
			Title("Bind style").
			Options(
				huh.NewOption("bind", BindStyleBind),
				huh.NewOption("bindd", BindStyleBindd),
			).
			Value(&state.BindStyle),
		huh.NewInput().
			Title("Key segment").
			Description("Example: `$mainMod, M`").
			Value(&state.BindKey).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return errors.New("bind key is required")
				}
				return nil
			}),
	).Title("Bind Options").WithHideFunc(func() bool {
		return !shouldGenerateBind(cfg.Format, state)
	})

	bindDescGroup := huh.NewGroup(
		huh.NewInput().
			Title("Bind description").
			Description("Used only for `bindd`.").
			Value(&state.BindDescription).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return errors.New("bind description is required for bindd")
				}
				return nil
			}),
	).Title("Bind Description").WithHideFunc(func() bool {
		return !shouldGenerateBind(cfg.Format, state) || state.BindStyle != BindStyleBindd
	})

	previewGroup := huh.NewGroup(
		huh.NewNote().
			Title("Preview").
			DescriptionFunc(func() string {
				return previewMarkdown(cfg.Format, *state)
			}, state).
			Next(true).
			NextLabel("Generate"),
	).Title("Preview")

	form := huh.NewForm(
		intentGroup,
		targetGroup,
		launchGroup,
		customGroup,
		specialGroup,
		advancedGroup,
		bindAskGroup,
		bindGroup,
		bindDescGroup,
		previewGroup,
	).WithAccessible(cfg.Accessible || os.Getenv("TERM") == "dumb")

	if cfg.Output != nil {
		form.WithOutput(cfg.Output)
	}
	if theme := selectTheme(cfg.Theme); theme != nil {
		form.WithTheme(theme)
	}
	form.WithShowHelp(true)
	form.WithShowErrors(true)
	return form
}

func commandKindForForm(state *State) string {
	if state.Intent == IntentFocusOnly {
		return CommandFocus
	}
	if state.Intent == IntentCustom && state.CustomCommandKind == CommandFocus {
		return CommandFocus
	}
	return CommandRun
}

func needsSpecialWorkspace(state *State) bool {
	if state.Intent == IntentScratchApp {
		state.UseScratch = true
		return true
	}
	if state.Intent == IntentCustom && state.WorkspaceMode == WorkspaceSpecial {
		return true
	}
	return false
}

func shouldGenerateBind(format string, state *State) bool {
	if format == FormatBind {
		state.GenerateBind = true
		return true
	}
	return state.GenerateBind
}

func previewMarkdown(format string, state State) string {
	outputs, err := state.Outputs(normalizeFormat(format))
	if err != nil {
		return fmt.Sprintf("`%v`", err)
	}

	lines := []string{
		"**Behavior**",
		outputs.Summary,
		"",
		"**Command**",
		fmt.Sprintf("```bash\n%s\n```", outputs.Command),
	}
	if outputs.Bind != "" && format != FormatCommand {
		lines = append(lines, "", "**Hyprland Bind**", fmt.Sprintf("```ini\n%s\n```", outputs.Bind))
	}
	return strings.Join(lines, "\n")
}

func selectTheme(name string) huh.Theme {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "charm":
		return huh.ThemeFunc(huh.ThemeCharm)
	case "dracula":
		return huh.ThemeFunc(huh.ThemeDracula)
	case "catppuccin":
		return huh.ThemeFunc(huh.ThemeCatppuccin)
	case "base16":
		return huh.ThemeFunc(huh.ThemeBase16)
	case "base":
		return huh.ThemeFunc(huh.ThemeBase)
	default:
		return huh.ThemeFunc(huh.ThemeCharm)
	}
}

func normalizeFormat(format string) string {
	switch format {
	case FormatCommand, FormatBind, FormatBoth:
		return format
	default:
		return FormatBoth
	}
}
