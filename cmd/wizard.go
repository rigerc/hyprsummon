package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/x/term"
	"github.com/rigerc/hyprsummon/internal/setup"
	"github.com/spf13/cobra"
)

func newSetupCmd() *cobra.Command {
	cfg := setup.Config{Format: setup.FormatBoth, Theme: "charm"}
	hyprctlPath := "hyprctl"

	cmd := &cobra.Command{
		Use:           "setup",
		Aliases:       []string{"wizard"},
		Short:         "Discover an application and generate a validated Hyprland Lua bind",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cfg.Force && cfg.OutputPath == "" {
				return errors.New("--force requires --output")
			}
			cfg.Format = normalizeSetupFormat(cfg.Format)
			cfg.Accessible = cfg.Accessible || cfg.NoColor || strings.EqualFold(os.Getenv("TERM"), "dumb") || !isTerminalReader(cmd.InOrStdin())
			cfg.Input = cmd.InOrStdin()
			cfg.PromptOutput = cmd.ErrOrStderr()

			client, err := newClient()
			if err != nil {
				return err
			}
			executable, err := resolvedExecutable()
			if err != nil {
				return fmt.Errorf("resolve hyprsummon executable: %w", err)
			}
			outputs, err := setup.Run(cmd.Context(), cfg, setup.Runtime{
				Client:     client,
				Prompter:   setup.NewHuhPrompter(cfg),
				Validator:  setup.LuaValidator{HyprctlPath: hyprctlPath},
				Executable: executable,
			})
			if errors.Is(err, setup.ErrCanceled) {
				return errors.New("setup canceled")
			}
			if err != nil {
				return err
			}

			printSetupOutputs(cmd, cfg, outputs)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&cfg.Format, "format", setup.FormatBoth, "output format: command, bind, or both")
	flags.StringVar(&cfg.BindKey, "bind-key", "", "prefill the Lua bind keys, for example 'SUPER + M'")
	flags.StringVar(&cfg.BindDesc, "bind-desc", "", "prefill the optional bind description")
	flags.BoolVar(&cfg.Advanced, "advanced", false, "always show advanced options")
	flags.BoolVar(&cfg.Accessible, "accessible", false, "use accessible prompt mode")
	flags.BoolVar(&cfg.NoColor, "no-color", false, "disable color and use plain prompts")
	flags.StringVar(&cfg.Theme, "theme", "charm", "setup theme: charm, dracula, catppuccin, base16, or base")
	flags.StringVar(&hyprctlPath, "hyprctl", "hyprctl", "path to hyprctl for non-mutating Lua validation")
	flags.StringVar(&cfg.OutputPath, "output", "", "write the generated command or Lua bind to this file")
	flags.BoolVar(&cfg.Force, "force", false, "overwrite an existing regular output file")
	return cmd
}

func normalizeSetupFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case setup.FormatCommand:
		return setup.FormatCommand
	case setup.FormatBind:
		return setup.FormatBind
	case "", setup.FormatBoth:
		return setup.FormatBoth
	default:
		return setup.FormatBoth
	}
}

func printSetupOutputs(cmd *cobra.Command, cfg setup.Config, outputs setup.Outputs) {
	out := cmd.OutOrStdout()

	switch cfg.Format {
	case setup.FormatCommand:
		_, _ = fmt.Fprintln(out, outputs.Command)
	case setup.FormatBind:
		_, _ = fmt.Fprintln(out, outputs.Bind)
	default:
		_, _ = fmt.Fprintf(out, "%s\n%s\n\n", styleLabel("Summary:"), outputs.Summary)
		_, _ = fmt.Fprintf(out, "%s\n%s\n", styleLabel("Command:"), outputs.Command)
		if outputs.Bind != "" {
			_, _ = fmt.Fprintf(out, "\n%s\n%s\n", styleLabel("Hyprland Lua bind:"), outputs.Bind)
		}
	}
	if cfg.OutputPath != "" {
		_, _ = fmt.Fprintf(out, "\n%s %s\n", styleLabel("Written to:"), cfg.OutputPath)
	}
}

func resolvedExecutable() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}

func isTerminalReader(reader io.Reader) bool {
	file, ok := reader.(interface{ Fd() uintptr })
	return ok && term.IsTerminal(file.Fd())
}
