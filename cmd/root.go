package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	hyprland "github.com/thiagokokada/hyprland-go"

	"github.com/rigerc/hyprsummon/internal/hypr"
	notifycmd "github.com/rigerc/hyprsummon/internal/notify"
	"github.com/spf13/cobra"
)

func Execute(args []string) int {
	root := newRootCmd()
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	return 0
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "hyprsummon",
		Short:         "Run or raise applications in Hyprland",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return errors.New("a command is required: run, focus, or wizard")
		},
	}

	root.AddCommand(newRunCmd())
	root.AddCommand(newFocusCmd())
	root.AddCommand(newWizardCmd())
	helpCmd := newHelpCmd(root)
	root.SetHelpCommand(helpCmd)
	root.AddCommand(helpCmd)
	root.SetHelpFunc(renderHelp)

	return root
}

func newRunCmd() *cobra.Command {
	opts := hypr.Options{}
	cmd := &cobra.Command{
		Use:           "run --class CLASS [flags] -- command [args...]",
		Short:         "Focus a matching window, or launch when no match exists",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Launch = append([]string(nil), args...)
			return runWithOptions(cmd, opts)
		},
	}

	registerCommonFlags(cmd, &opts)
	flags := cmd.Flags()
	flags.BoolVar(&opts.Background, "background", false, "launch without waiting for or focusing a new window")
	flags.BoolVar(&opts.ShowSpecialAfterLaunch, "show-special-after-launch", false, "reveal the named special workspace after launching a new match there")
	_ = cmd.MarkFlagRequired("class")
	cmd.MarkFlagsMutuallyExclusive("all-workspaces", "current-workspace-only")
	cmd.MarkFlagsMutuallyExclusive("prefer-floating", "prefer-tiled")
	cmd.MarkFlagsMutuallyExclusive("prefer-special", "exclude-special")
	cmd.MarkFlagsMutuallyExclusive("fullscreen", "maximize")
	return cmd
}

func newFocusCmd() *cobra.Command {
	opts := hypr.Options{}
	cmd := &cobra.Command{
		Use:           "focus --class CLASS [flags]",
		Short:         "Focus a matching window without launching",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithOptions(cmd, opts)
		},
	}

	registerCommonFlags(cmd, &opts)
	_ = cmd.MarkFlagRequired("class")
	cmd.MarkFlagsMutuallyExclusive("all-workspaces", "current-workspace-only")
	cmd.MarkFlagsMutuallyExclusive("prefer-floating", "prefer-tiled")
	cmd.MarkFlagsMutuallyExclusive("prefer-special", "exclude-special")
	cmd.MarkFlagsMutuallyExclusive("fullscreen", "maximize")
	return cmd
}

func registerCommonFlags(cmd *cobra.Command, opts *hypr.Options) {
	flags := cmd.Flags()
	flags.StringVar(&opts.Class, "class", "", "window class to match")
	flags.StringVar(&opts.Title, "title", "", "window title to match")
	flags.StringVar(&opts.InitialClass, "initial-class", "", "window initial class to match")
	flags.BoolVar(&opts.AllWorkspaces, "all-workspaces", false, "search across all workspaces")
	flags.BoolVar(&opts.CurrentWorkspaceOnly, "current-workspace-only", false, "only search for matches on the active workspace")
	flags.BoolVar(&opts.PreferFloating, "prefer-floating", false, "prefer floating matches when several windows match")
	flags.BoolVar(&opts.PreferTiled, "prefer-tiled", false, "prefer tiled matches when several windows match")
	flags.BoolVar(&opts.PreferSpecial, "prefer-special", false, "prefer matches on special workspaces")
	flags.BoolVar(&opts.ExcludeSpecial, "exclude-special", false, "ignore matches on special workspaces")
	flags.BoolVar(&opts.ToggleSpecial, "toggle-special", false, "reveal a matching special workspace before focusing it")
	flags.BoolVar(&opts.Pull, "pull", false, "move an existing match to the active workspace before focusing it")
	flags.BoolVar(&opts.MoveToSpecial, "move-to-special", false, "move an existing match to the named special workspace before focusing it")
	flags.StringVar(&opts.SpecialWorkspace, "special-workspace", "", "target named special workspace for special-workspace actions")
	flags.BoolVar(&opts.Scratch, "scratch", false, "high-level scratchpad mode for a named special workspace")
	flags.BoolVar(&opts.Cycle, "cycle", false, "cycle through multiple matching windows")
	flags.BoolVar(&opts.Fullscreen, "fullscreen", false, "fullscreen a focused match after selecting it")
	flags.BoolVar(&opts.Maximize, "maximize", false, "maximize a focused match after selecting it")
	flags.BoolVar(&opts.Notify, "notify", false, "send desktop notifications with notify-send when available")
	flags.BoolVar(&opts.Debug, "debug", false, "send diagnostic notifications with notify-send")
	flags.BoolVar(&opts.Verbose, "verbose", false, "print diagnostic output to stderr")
}

func runWithOptions(cmd *cobra.Command, opts hypr.Options) error {
	applyModes(cmd.Name(), &opts)

	if err := opts.Validate(); err != nil {
		return err
	}

	client, err := newClient()
	if err != nil {
		return err
	}

	runner := hypr.Runner{
		Client:   client,
		Notifier: notifycmd.New(),
		StdErr:   cmd.ErrOrStderr(),
		Now:      time.Now,
		Sleep:    time.Sleep,
	}

	return runner.Run(context.Background(), opts)
}

func applyModes(commandName string, opts *hypr.Options) {
	if !opts.Scratch {
		return
	}

	opts.PreferSpecial = true
	opts.ToggleSpecial = true
	opts.MoveToSpecial = true
	if commandName == "run" {
		opts.ShowSpecialAfterLaunch = true
	}
}

func newClient() (*hyprland.RequestClient, error) {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		return nil, errors.New("XDG_RUNTIME_DIR is not set")
	}

	instance := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
	socket, err := resolveSocketPath(runtimeDir, instance)
	if err != nil {
		return nil, err
	}

	return hyprland.NewClient(socket), nil
}

func resolveSocketPath(runtimeDir string, instance string) (string, error) {
	if instance != "" {
		return filepath.Join(runtimeDir, "hypr", instance, ".socket.sock"), nil
	}

	return discoverSocketPath(runtimeDir)
}

func discoverSocketPath(runtimeDir string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(runtimeDir, "hypr", "*", ".socket.sock"))
	if err != nil {
		return "", fmt.Errorf("finding Hyprland sockets: %w", err)
	}
	if len(matches) == 0 {
		return "", errors.New("HYPRLAND_INSTANCE_SIGNATURE is not set and no Hyprland socket was found")
	}
	if len(matches) == 1 {
		return matches[0], nil
	}

	type candidate struct {
		path    string
		modTime time.Time
	}

	candidates := make([]candidate, 0, len(matches))
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{
			path:    match,
			modTime: info.ModTime(),
		})
	}
	if len(candidates) == 0 {
		return "", errors.New("HYPRLAND_INSTANCE_SIGNATURE is not set and no readable Hyprland socket was found")
	}

	sort.Slice(candidates, func(i int, j int) bool {
		if candidates[i].modTime.Equal(candidates[j].modTime) {
			return candidates[i].path > candidates[j].path
		}
		return candidates[i].modTime.After(candidates[j].modTime)
	})

	return candidates[0].path, nil
}
