package cmd

import (
	"errors"
	"fmt"
	"strings"

	huh "charm.land/huh/v2"
	"github.com/rigerc/hyprsummon/internal/wizard"
	"github.com/spf13/cobra"
)

func newWizardCmd() *cobra.Command {
	cfg := wizard.Config{Format: wizard.FormatBoth, Theme: "charm"}

	cmd := &cobra.Command{
		Use:           "wizard",
		Short:         "Generate a hyprsummon command and optional Hyprland bind interactively",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.Format = normalizeWizardFormat(cfg.Format)
			cfg.Output = cmd.OutOrStdout()

			outputs, err := wizard.Run(cfg)
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					return errors.New("wizard aborted")
				}
				return err
			}

			printWizardOutputs(cmd, cfg.Format, outputs)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&cfg.Format, "format", wizard.FormatBoth, "output format: command, bind, or both")
	flags.StringVar(&cfg.BindKey, "bind-key", "", "prefill the bind key segment, for example '$mainMod, M'")
	flags.StringVar(&cfg.BindDesc, "bind-desc", "", "prefill the bind description for bindd output")
	flags.BoolVar(&cfg.Advanced, "advanced", false, "always show advanced options")
	flags.BoolVar(&cfg.Accessible, "accessible", false, "use accessible prompt mode")
	flags.StringVar(&cfg.Theme, "theme", "charm", "wizard theme: charm, dracula, catppuccin, base16, or base")
	return cmd
}

func normalizeWizardFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case wizard.FormatCommand:
		return wizard.FormatCommand
	case wizard.FormatBind:
		return wizard.FormatBind
	case "", wizard.FormatBoth:
		return wizard.FormatBoth
	default:
		return wizard.FormatBoth
	}
}

func printWizardOutputs(cmd *cobra.Command, format string, outputs wizard.Outputs) {
	out := cmd.OutOrStdout()

	switch format {
	case wizard.FormatCommand:
		_, _ = fmt.Fprintln(out, outputs.Command)
	case wizard.FormatBind:
		_, _ = fmt.Fprintln(out, outputs.Bind)
	default:
		_, _ = fmt.Fprintf(out, "%s\n%s\n\n", styleLabel("Summary:"), outputs.Summary)
		_, _ = fmt.Fprintf(out, "%s\n%s\n", styleLabel("Command:"), outputs.Command)
		if outputs.Bind != "" {
			_, _ = fmt.Fprintf(out, "\n%s\n%s\n", styleLabel("Bind:"), outputs.Bind)
		}
	}
}
