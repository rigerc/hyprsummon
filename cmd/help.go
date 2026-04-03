package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type flagHelp struct {
	summary      string
	details      string
	defaultValue string
	example      string
}

var flagHelpText = map[string]flagHelp{
	"all-workspaces": {
		summary:      "Search the full Hyprland client list instead of relying on active-workspace preference.",
		details:      "This flag is mainly for explicitness in scripts and aliases. The default behavior already falls back to other workspaces when no current-workspace match exists.",
		defaultValue: "false",
		example:      "hyprsummon run --class kitty --all-workspaces -- kitty",
	},
	"background": {
		summary:      "Launch the command and return immediately.",
		details:      "When no match exists, hyprsummon dispatches the launch command and exits without polling for a new window. It also skips post-launch focus, fullscreen, and maximize actions.",
		defaultValue: "false",
		example:      "hyprsummon run --class steam --background -- steam",
	},
	"class": {
		summary:      "Required exact-match window class selector.",
		details:      "Every invocation of run/focus must provide --class. Existing clients only match when their Hyprland class exactly equals this value.",
		defaultValue: "",
		example:      "hyprsummon run --class kitty -- kitty",
	},
	"current-workspace-only": {
		summary:      "Restrict matching to the active workspace.",
		details:      "If no matching window exists on the current workspace, run behaves as if nothing is running and uses the launch command after --. Focus returns an error.",
		defaultValue: "false",
		example:      "hyprsummon focus --class kitty --current-workspace-only",
	},
	"cycle": {
		summary:      "Cycle through multiple matching windows instead of always taking the first one.",
		details:      "The cycle order is derived from the ordered match list for the current invocation. That ordered list can be affected by workspace and preference flags.",
		defaultValue: "false",
		example:      "hyprsummon run --class kitty --cycle -- kitty",
	},
	"debug": {
		summary:      "Send diagnostic trace messages using notify-send.",
		details:      "Debug mode emits internal trace messages like dispatch strings and timeout notes as desktop notifications. Combine it with --verbose if you also want the same diagnostics on stderr.",
		defaultValue: "false",
		example:      "hyprsummon run --class kitty --debug -- kitty",
	},
	"exclude-special": {
		summary:      "Ignore windows that live on Hyprland special workspaces.",
		details:      "Use this when scratchpad or special-workspace instances should never count as matches. This flag is mutually exclusive with --prefer-special.",
		defaultValue: "false",
		example:      "hyprsummon run --class kitty --exclude-special -- kitty",
	},
	"fullscreen": {
		summary:      "Apply Hyprland fullscreen mode after focusing the selected window.",
		details:      "This dispatches `fullscreen 0 set` after hyprsummon focuses a window. It is mutually exclusive with --maximize.",
		defaultValue: "false",
		example:      "hyprsummon run --class firefox --fullscreen -- firefox",
	},
	"initial-class": {
		summary:      "Optional exact-match selector for Hyprland initialClass.",
		details:      "Use this to narrow matches when multiple windows share the same runtime class or when an application mutates its class after launch.",
		defaultValue: "",
		example:      "hyprsummon run --class firefox --initial-class firefox -- firefox",
	},
	"maximize": {
		summary:      "Apply Hyprland maximize mode after focusing the selected window.",
		details:      "This dispatches `fullscreen 1 set` after hyprsummon focuses a window. It is mutually exclusive with --fullscreen.",
		defaultValue: "false",
		example:      "hyprsummon run --class firefox --maximize -- firefox",
	},
	"move-to-special": {
		summary:      "Move an existing match to the named special workspace before focusing it.",
		details:      "Use this with --special-workspace when that workspace is the canonical home for an app. Hyprsummon dispatches movetoworkspacesilent to the target special workspace before any reveal or focus step. It is mutually exclusive with --pull.",
		defaultValue: "false",
		example:      "hyprsummon run --class spotify --special-workspace music --move-to-special --toggle-special -- spotify-launcher",
	},
	"notify": {
		summary:      "Send desktop notifications using notify-send when available.",
		details:      "Notifications are best-effort only. If notify-send is missing or fails, the main Hyprland action still succeeds.",
		defaultValue: "false",
		example:      "hyprsummon run --class discord --notify -- discord",
	},
	"prefer-floating": {
		summary:      "Prefer floating matches over tiled matches.",
		details:      "When several windows match, floating windows are ordered before non-floating windows. It is mutually exclusive with --prefer-tiled.",
		defaultValue: "false",
		example:      "hyprsummon run --class kitty --prefer-floating -- kitty",
	},
	"prefer-special": {
		summary:      "Prefer matches that live on special workspaces.",
		details:      "Special-workspace matches are ordered before regular-workspace matches. It is mutually exclusive with --exclude-special.",
		defaultValue: "false",
		example:      "hyprsummon run --class kitty --prefer-special --toggle-special -- kitty",
	},
	"prefer-tiled": {
		summary:      "Prefer tiled matches over floating matches.",
		details:      "When several windows match, tiled windows are ordered before floating windows. It is mutually exclusive with --prefer-floating.",
		defaultValue: "false",
		example:      "hyprsummon run --class kitty --prefer-tiled -- kitty",
	},
	"pull": {
		summary:      "Move an existing match to the active workspace before focusing it.",
		details:      "When the selected window is on another workspace, hyprsummon dispatches movetoworkspace for that window and then focuses it.",
		defaultValue: "false",
		example:      "hyprsummon run --class firefox --pull -- firefox",
	},
	"scratch": {
		summary:      "Enable high-level scratchpad mode for a named special workspace.",
		details:      "Scratch mode expands to --prefer-special --move-to-special --toggle-special, and in `run` it also enables --show-special-after-launch. Use it with --special-workspace when an app should live on a named special workspace and repeated invocation should hide it when already focused there.",
		defaultValue: "false",
		example:      "hyprsummon run --class spotify --special-workspace music --scratch -- spotify-launcher",
	},
	"special-workspace": {
		summary:      "Name the target special workspace for launch and special-workspace actions.",
		details:      "In `run`, hyprsummon formats launch as `exec [workspace special:NAME silent] <command>`. With --move-to-special, --toggle-special, --show-special-after-launch, or --scratch, this also becomes the target workspace for existing-window relocation and visibility handling.",
		defaultValue: "",
		example:      "hyprsummon run --class spotify --special-workspace music -- spotify",
	},
	"show-special-after-launch": {
		summary:      "Reveal the named special workspace after launching a new match there.",
		details:      "This only affects launches in `run`. After dispatching the special-workspace launch and detecting the new matching window, hyprsummon toggles the target special workspace visible if it is currently hidden.",
		defaultValue: "false",
		example:      "hyprsummon run --class spotify --special-workspace music --show-special-after-launch -- spotify-launcher",
	},
	"title": {
		summary:      "Optional exact-match selector for current window title.",
		details:      "Use this when several windows share a class but you want one specific title. Matching is exact and case-sensitive.",
		defaultValue: "",
		example:      "hyprsummon run --class kitty --title scratch -- kitty --title scratch",
	},
	"toggle-special": {
		summary:      "Toggle visibility for the target special workspace around the selected match.",
		details:      "If the selected match lives on the target special workspace and that workspace is hidden, hyprsummon reveals it before focusing. If the selected match is already focused on a visible target special workspace, hyprsummon hides that workspace instead of doing nothing.",
		defaultValue: "false",
		example:      "hyprsummon run --class spotify --special-workspace music --move-to-special --toggle-special -- spotify-launcher",
	},
	"verbose": {
		summary:      "Print diagnostic information to stderr.",
		details:      "Verbose mode shows the same diagnostics on stderr. Combine it with --debug if you also want notify-send popups.",
		defaultValue: "false",
		example:      "hyprsummon run --class kitty --verbose -- kitty",
	},
}

func newHelpCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:                "help [command] | help flag <flag>",
		Short:              "Help about commands or one specific flag",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return root.Help()
			}

			if args[0] == "flag" {
				if len(args) != 2 {
					return fmt.Errorf("help flag requires exactly one flag name")
				}
				return printFlagHelp(root, cmd, args[1])
			}

			target, _, err := root.Find(args)
			if err == nil && target != nil {
				return target.Help()
			}

			return fmt.Errorf("unknown help topic %q", strings.Join(args, " "))
		},
	}
}

func printFlagHelp(root *cobra.Command, cmd *cobra.Command, topic string) error {
	name := strings.TrimLeft(topic, "-")
	flag, owners := lookupFlag(root, name)
	if flag == nil {
		return fmt.Errorf("unknown flag help topic %q", topic)
	}

	entry, ok := flagHelpText[name]
	if !ok {
		return fmt.Errorf("detailed help for %q is not defined", topic)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n", styleBold(formatFlagHeading(flag.Name)))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", styleLabel("Summary:"), entry.summary)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", styleLabel("Usage:"), styleValue("--"+flag.Name))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", styleLabel("Commands:"), strings.Join(owners, ", "))
	if flag.Value.Type() != "bool" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", styleLabel("Value:"), flag.Value.Type())
	}
	if entry.defaultValue != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", styleLabel("Default:"), entry.defaultValue)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", styleLabel("Details:"), entry.details)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", styleLabel("Example:"), styleValue(entry.example))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%s %s\n", styleDim("Available flag topics:"), strings.Join(availableFlagTopics(), ", "))
	return nil
}

func formatFlagHeading(name string) string {
	return "--" + name
}

func availableFlagTopics() []string {
	topics := make([]string, 0, len(flagHelpText))
	for name := range flagHelpText {
		topics = append(topics, "--"+name)
	}
	sort.Strings(topics)
	return topics
}

func lookupFlag(root *cobra.Command, name string) (*pflag.Flag, []string) {
	run := findSubcommand(root, "run")
	focus := findSubcommand(root, "focus")
	if run == nil || focus == nil {
		return nil, nil
	}

	owners := make([]string, 0, 2)
	var found *pflag.Flag
	if f := run.Flags().Lookup(name); f != nil {
		owners = append(owners, "run")
		found = f
	}
	if f := focus.Flags().Lookup(name); f != nil {
		owners = append(owners, "focus")
		if found == nil {
			found = f
		}
	}
	return found, owners
}

func findSubcommand(root *cobra.Command, name string) *cobra.Command {
	for _, sub := range root.Commands() {
		if sub.Name() == name {
			return sub
		}
	}
	return nil
}

func renderHelp(cmd *cobra.Command, _ []string) {
	out := cmd.OutOrStdout()

	if cmd.Short != "" {
		_, _ = fmt.Fprintf(out, "%s\n\n", styleBold(cmd.Short))
	}
	_, _ = fmt.Fprintf(out, "%s\n  %s\n", styleLabel("Usage:"), styleValue(cmd.UseLine()))

	if cmd.HasAvailableSubCommands() {
		_, _ = fmt.Fprintf(out, "\n%s\n", styleLabel("Commands:"))
		for _, sub := range cmd.Commands() {
			if !sub.IsAvailableCommand() || sub.IsAdditionalHelpTopicCommand() {
				continue
			}
			_, _ = fmt.Fprintf(out, "  %-18s %s\n", styleValue(sub.Name()), sub.Short)
		}
	}

	if cmd.Name() == "hyprsummon" {
		run := findSubcommand(cmd, "run")
		focus := findSubcommand(cmd, "focus")
		if run != nil && focus != nil {
			_, _ = fmt.Fprintf(out, "\n%s\n", styleLabel("Common Flags (run, focus):"))
			printSharedFlags(out, run.LocalFlags(), focus.LocalFlags())
			_, _ = fmt.Fprintf(out, "\n%s\n", styleLabel("Run-only Flags:"))
			printUniqueFlags(out, run.LocalFlags(), focus.LocalFlags())
		}
	} else {
		printFlagTable(out, cmd.LocalFlags())
	}

	_, _ = fmt.Fprintf(out, "\n%s %s\n", styleDim("Detailed flag help:"), styleValue(cmd.Root().Name()+" help flag --class"))
}

func printSharedFlags(out anyWriter, left *pflag.FlagSet, right *pflag.FlagSet) {
	rightNames := map[string]struct{}{}
	right.VisitAll(func(flag *pflag.Flag) {
		rightNames[flag.Name] = struct{}{}
	})
	left.VisitAll(func(flag *pflag.Flag) {
		if _, ok := rightNames[flag.Name]; !ok {
			return
		}
		printFlagLine(out, flag)
	})
}

func printUniqueFlags(out anyWriter, left *pflag.FlagSet, right *pflag.FlagSet) {
	rightNames := map[string]struct{}{}
	right.VisitAll(func(flag *pflag.Flag) {
		rightNames[flag.Name] = struct{}{}
	})
	left.VisitAll(func(flag *pflag.Flag) {
		if _, ok := rightNames[flag.Name]; ok {
			return
		}
		printFlagLine(out, flag)
	})
}

func printFlagTable(out anyWriter, flags *pflag.FlagSet) {
	if flags == nil || !flags.HasAvailableFlags() {
		return
	}
	_, _ = fmt.Fprintf(out, "\n%s\n", styleLabel("Flags:"))
	flags.VisitAll(func(flag *pflag.Flag) {
		printFlagLine(out, flag)
	})
}

func printFlagLine(out anyWriter, flag *pflag.Flag) {
	name := "--" + flag.Name
	if flag.Shorthand != "" {
		name = fmt.Sprintf("-%s, %s", flag.Shorthand, name)
	}
	suffix := ""
	if flag.Value.Type() != "bool" {
		suffix = " <" + flag.Value.Type() + ">"
	}
	line := fmt.Sprintf("  %-28s %s", styleValue(name+suffix), flag.Usage)
	if flag.DefValue != "" && flag.DefValue != "false" {
		line += " " + styleDim("(default: "+flag.DefValue+")")
	}
	_, _ = fmt.Fprintln(out, line)
}

type anyWriter interface {
	Write(p []byte) (n int, err error)
}
