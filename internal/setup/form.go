package setup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	huh "charm.land/huh/v2"
)

type HuhPrompter struct {
	cfg Config
}

func NewHuhPrompter(cfg Config) *HuhPrompter {
	if (cfg.Accessible || cfg.NoColor) && cfg.Input != nil {
		cfg.Input = &promptByteReader{reader: cfg.Input}
	}
	return &HuhPrompter{cfg: cfg}
}

type promptByteReader struct {
	reader io.Reader
}

func (r *promptByteReader) Read(buffer []byte) (int, error) {
	if len(buffer) > 1 {
		buffer = buffer[:1]
	}
	return r.reader.Read(buffer)
}

func (p *HuhPrompter) ChooseApp(ctx context.Context, apps []AppChoice) (string, error) {
	options := make([]huh.Option[string], 0, len(apps)+2)
	for _, app := range apps {
		options = append(options, huh.NewOption(app.Label(), app.Class))
	}
	options = append(options,
		huh.NewOption("Enter a window class manually", ChoiceManual),
		huh.NewOption("Refresh running windows", ChoiceRefresh),
	)
	var choice string
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Choose the application to summon").
			Description("Classes come from windows currently known to Hyprland.").
			Options(options...).
			Value(&choice),
	))
	if err := p.runForm(ctx, form); err != nil {
		return "", err
	}
	return choice, nil
}

func (p *HuhPrompter) Configure(ctx context.Context, state *State) error {
	if p.cfg.Accessible || p.cfg.NoColor {
		return p.configureAccessible(ctx, state)
	}
	return p.runForm(ctx, BuildForm(p.cfg, state))
}

func (p *HuhPrompter) configureAccessible(ctx context.Context, state *State) error {
	forms := []*huh.Form{
		huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title("What do you want to set up?").
				Options(
					huh.NewOption("Run or raise", IntentRunOrRaise),
					huh.NewOption("Focus existing only", IntentFocusOnly),
					huh.NewOption("Bring existing window here", IntentBringHere),
					huh.NewOption("Scratch app on a special workspace", IntentScratchApp),
					huh.NewOption("Custom", IntentCustom),
				).
				Value(&state.Intent),
		)),
	}
	for _, form := range forms {
		if err := p.runForm(ctx, form); err != nil {
			return err
		}
	}

	if state.Intent == IntentCustom {
		form := huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title("Base command").
				Options(huh.NewOption("run", CommandRun), huh.NewOption("focus", CommandFocus)).
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
		))
		if err := p.runForm(ctx, form); err != nil {
			return err
		}
	}

	target := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Window class").Value(&state.Class).Validate(requiredInput("class")),
		huh.NewInput().Title("Window title (optional)").Value(&state.Title),
		huh.NewInput().Title("Initial class (optional)").Value(&state.InitialClass),
	))
	if err := p.runForm(ctx, target); err != nil {
		return err
	}

	if commandKindForForm(state) == CommandRun {
		launch := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Launch command").Value(&state.LaunchCommand).Validate(requiredInput("launch command")),
		))
		if err := p.runForm(ctx, launch); err != nil {
			return err
		}
	}

	if needsSpecialWorkspace(state) {
		special := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Special workspace name").Value(&state.SpecialWorkspace).Validate(requiredInput("special workspace")),
			huh.NewConfirm().Title("Use scratch mode?").Value(&state.UseScratch),
		))
		if err := p.runForm(ctx, special); err != nil {
			return err
		}
	}

	if p.cfg.Advanced || state.Intent == IntentCustom {
		advanced := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().Title("Cycle through multiple matches?").Value(&state.Cycle),
			huh.NewSelect[string]().Title("Match preference").Options(
				huh.NewOption("None", PreferenceNone),
				huh.NewOption("Prefer floating", PreferenceFloating),
				huh.NewOption("Prefer tiled", PreferenceTiled),
				huh.NewOption("Prefer special", PreferencePreferSpecial),
				huh.NewOption("Exclude special", PreferenceExcludeSpecial),
			).Value(&state.Preference),
			huh.NewSelect[string]().Title("Post-focus mode").Options(
				huh.NewOption("None", FocusModeNone),
				huh.NewOption("Fullscreen", FocusModeFullscreen),
				huh.NewOption("Maximize", FocusModeMaximize),
			).Value(&state.FocusMode),
			huh.NewConfirm().Title("Notify?").Value(&state.Notify),
			huh.NewConfirm().Title("Verbose stderr diagnostics?").Value(&state.Verbose),
			huh.NewConfirm().Title("Desktop debug notifications?").Value(&state.Debug),
		))
		if err := p.runForm(ctx, advanced); err != nil {
			return err
		}
	}

	if p.cfg.Format == FormatBoth {
		ask := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().Title("Generate a Hyprland Lua bind?").Value(&state.GenerateBind),
		))
		if err := p.runForm(ctx, ask); err != nil {
			return err
		}
	} else {
		state.GenerateBind = p.cfg.Format == FormatBind
	}
	if state.GenerateBind {
		bind := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Key combination").Value(&state.BindKey).Validate(requiredInput("bind key")),
			huh.NewInput().Title("Bind description (optional)").Value(&state.BindDescription),
		))
		if err := p.runForm(ctx, bind); err != nil {
			return err
		}
	}
	return nil
}

func BuildForm(cfg Config, state *State) *huh.Form {
	intentGroup := huh.NewGroup(
		huh.NewSelect[string]().
			Title("What do you want to set up?").
			Description("Start from a preset and let setup generate the command.").
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
			Validate(requiredInput("class")),
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
			Description("Shell command used when no matching window exists.").
			Value(&state.LaunchCommand).
			Validate(requiredInput("launch command")),
	).Title("Launch").WithHideFunc(func() bool { return commandKindForForm(state) != CommandRun })

	customGroup := huh.NewGroup(
		huh.NewSelect[string]().
			Title("Base command").
			Options(huh.NewOption("run", CommandRun), huh.NewOption("focus", CommandFocus)).
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
	).Title("Custom").WithHideFunc(func() bool { return state.Intent != IntentCustom })

	specialGroup := huh.NewGroup(
		huh.NewInput().
			Title("Special workspace name").
			Description("Use the bare name, not special:NAME.").
			Value(&state.SpecialWorkspace).
			Validate(requiredInput("special workspace")),
		huh.NewConfirm().
			Title("Use scratch mode?").
			Description("Recommended for app toggles such as music players and scratch terminals.").
			Value(&state.UseScratch),
	).Title("Special Workspace").WithHideFunc(func() bool { return !needsSpecialWorkspace(state) })

	advancedGroup := huh.NewGroup(
		huh.NewConfirm().Title("Cycle through multiple matches?").Value(&state.Cycle),
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
	).Title("Advanced").WithHideFunc(func() bool { return !(cfg.Advanced || state.Intent == IntentCustom) })

	bindAskGroup := huh.NewGroup(
		huh.NewConfirm().Title("Generate a Hyprland Lua bind?").Value(&state.GenerateBind),
	).Title("Bind").WithHideFunc(func() bool { return cfg.Format != FormatBoth })

	bindGroup := huh.NewGroup(
		huh.NewInput().
			Title("Key combination").
			Description("Example: SUPER + M").
			Value(&state.BindKey).
			Validate(requiredInput("bind key")),
		huh.NewInput().
			Title("Bind description").
			Description("Optional description shown by hyprctl binds.").
			Value(&state.BindDescription),
	).Title("Bind Options").WithHideFunc(func() bool { return !shouldGenerateBind(cfg.Format, state) })

	previewGroup := huh.NewGroup(
		huh.NewNote().
			Title("Preview").
			DescriptionFunc(func() string { return previewMarkdown(cfg.Format, *state) }, state).
			Next(true).
			NextLabel("Validate"),
	).Title("Preview")

	return huh.NewForm(
		intentGroup,
		targetGroup,
		launchGroup,
		customGroup,
		specialGroup,
		advancedGroup,
		bindAskGroup,
		bindGroup,
		previewGroup,
	).WithShowHelp(true).WithShowErrors(true)
}

func (p *HuhPrompter) runForm(ctx context.Context, form *huh.Form) error {
	if p.cfg.Input != nil {
		form.WithInput(p.cfg.Input)
	}
	if p.cfg.PromptOutput != nil {
		form.WithOutput(p.cfg.PromptOutput)
	}
	plain := p.cfg.Accessible || p.cfg.NoColor
	form.WithAccessible(plain)
	if plain {
		form.WithTheme(huh.ThemeFunc(huh.ThemeBase))
	} else if theme := selectTheme(p.cfg.Theme); theme != nil {
		form.WithTheme(theme)
	}
	return normalizePromptError(ctx, form.RunWithContext(ctx))
}

func normalizePromptError(ctx context.Context, err error) error {
	if errors.Is(err, huh.ErrUserAborted) || ctx.Err() != nil {
		return ErrCanceled
	}
	return err
}

func requiredInput(name string) func(string) error {
	return func(value string) error {
		if strings.TrimSpace(value) == "" {
			return errors.New(name + " is required")
		}
		return nil
	}
}

func commandKindForForm(state *State) string {
	if state.Intent == IntentFocusOnly || (state.Intent == IntentCustom && state.CustomCommandKind == CommandFocus) {
		return CommandFocus
	}
	return CommandRun
}

func needsSpecialWorkspace(state *State) bool {
	if state.Intent == IntentScratchApp {
		state.UseScratch = true
		return true
	}
	return state.Intent == IntentCustom && state.WorkspaceMode == WorkspaceSpecial
}

func shouldGenerateBind(format string, state *State) bool {
	if format == FormatCommand {
		state.GenerateBind = false
		return false
	}
	if format == FormatBind {
		state.GenerateBind = true
		return true
	}
	return state.GenerateBind
}

func previewMarkdown(format string, state State) string {
	outputs, err := state.Outputs(format)
	if err != nil {
		return fmt.Sprintf("`%v`", err)
	}
	lines := []string{"**Behavior**", outputs.Summary, "", "**Command**", fmt.Sprintf("```bash\n%s\n```", outputs.Command)}
	if outputs.Bind != "" && format != FormatCommand {
		lines = append(lines, "", "**Hyprland Lua Bind**", fmt.Sprintf("```lua\n%s\n```", outputs.Bind))
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
