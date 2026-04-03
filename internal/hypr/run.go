package hypr

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	hyprland "github.com/thiagokokada/hyprland-go"
)

const (
	pollInterval = 100 * time.Millisecond
	pollTimeout  = 3 * time.Second
)

type Client interface {
	Clients() ([]hyprland.Client, error)
	ActiveWindow() (hyprland.Window, error)
	ActiveWorkspace() (hyprland.Workspace, error)
	Monitors() ([]hyprland.Monitor, error)
	Dispatch(params ...string) ([]hyprland.Response, error)
}

type Notifier interface {
	Notify(summary string, body string) error
}

type Options struct {
	Class                  string
	Title                  string
	InitialClass           string
	AllWorkspaces          bool
	CurrentWorkspaceOnly   bool
	PreferFloating         bool
	PreferTiled            bool
	PreferSpecial          bool
	ExcludeSpecial         bool
	ToggleSpecial          bool
	Background             bool
	SpecialWorkspace       string
	MoveToSpecial          bool
	ShowSpecialAfterLaunch bool
	Pull                   bool
	Cycle                  bool
	Fullscreen             bool
	Maximize               bool
	Notify                 bool
	Debug                  bool
	Verbose                bool
	Scratch                bool
	Launch                 []string
}

func (o Options) Validate() error {
	if o.Class == "" {
		return errors.New("--class is required")
	}

	if o.SpecialWorkspace != "" && strings.HasPrefix(o.SpecialWorkspace, "special:") {
		return errors.New("--special-workspace should be a name without the special: prefix")
	}
	if o.MoveToSpecial && o.SpecialWorkspace == "" {
		return errors.New("--move-to-special requires --special-workspace")
	}
	if o.ShowSpecialAfterLaunch && o.SpecialWorkspace == "" {
		return errors.New("--show-special-after-launch requires --special-workspace")
	}
	if o.MoveToSpecial && o.Pull {
		return errors.New("--move-to-special and --pull cannot be used together")
	}
	if o.AllWorkspaces && o.CurrentWorkspaceOnly {
		return errors.New("--all-workspaces and --current-workspace-only cannot be used together")
	}
	if o.PreferFloating && o.PreferTiled {
		return errors.New("--prefer-floating and --prefer-tiled cannot be used together")
	}
	if o.PreferSpecial && o.ExcludeSpecial {
		return errors.New("--prefer-special and --exclude-special cannot be used together")
	}
	if o.Fullscreen && o.Maximize {
		return errors.New("--fullscreen and --maximize cannot be used together")
	}

	return nil
}

type Runner struct {
	Client   Client
	Notifier Notifier
	StdErr   io.Writer
	Now      func() time.Time
	Sleep    func(time.Duration)
}

func (r Runner) Run(ctx context.Context, opts Options) error {
	if r.Client == nil {
		return errors.New("client is required")
	}
	if r.StdErr == nil {
		r.StdErr = io.Discard
	}
	if r.Now == nil {
		r.Now = time.Now
	}
	if r.Sleep == nil {
		r.Sleep = time.Sleep
	}

	clients, err := r.Client.Clients()
	if err != nil {
		return fmt.Errorf("listing clients: %w", err)
	}

	activeWindow, err := r.Client.ActiveWindow()
	if err != nil {
		return fmt.Errorf("getting active window: %w", err)
	}

	activeWorkspace, err := r.Client.ActiveWorkspace()
	if err != nil {
		return fmt.Errorf("getting active workspace: %w", err)
	}

	monitors, err := r.Client.Monitors()
	if err != nil {
		return fmt.Errorf("listing monitors: %w", err)
	}

	matches := SelectCandidates(FilterClients(clients, opts), activeWorkspace.Id, opts)
	selected := SelectClient(matches, activeWindow.Address, opts.Cycle)
	if selected != nil {
		if activeWindow.Address == selected.Address && r.shouldHideSpecial(*selected, monitors, opts) {
			if err := r.hideSpecialWorkspace(opts, targetSpecialWorkspaceName(*selected, opts)); err != nil {
				return err
			}
			r.notify(opts, "hyprsummon", fmt.Sprintf("hid %s", selected.Class))
			return nil
		}

		if activeWindow.Address == selected.Address {
			r.debugf(opts, "match %s already focused", selected.Address)
			r.notify(opts, "hyprsummon", fmt.Sprintf("already focused %s", selected.Class))
			return nil
		}

		if err := r.focusExisting(*selected, activeWorkspace.Id, monitors, opts); err != nil {
			return err
		}

		return nil
	}

	if len(opts.Launch) == 0 {
		return errors.New("no matching window found and no launch command was provided after --")
	}

	if err := r.launch(opts); err != nil {
		return err
	}

	if opts.Background {
		r.notify(opts, "hyprsummon", fmt.Sprintf("started %s in background", opts.Class))
		return nil
	}

	before := make(map[string]struct{}, len(matches))
	for _, client := range matches {
		before[client.Address] = struct{}{}
	}

	started, err := r.waitForNewMatch(ctx, before, activeWorkspace.Id, opts)
	if err != nil {
		r.debugf(opts, "launch completed but no new match became available: %v", err)
		r.notify(opts, "hyprsummon", fmt.Sprintf("started %s", opts.Class))
		return nil
	}

	if opts.SpecialWorkspace == "" {
		if err := r.dispatchOne(opts, FormatFocus(started.Address)); err != nil {
			return fmt.Errorf("focusing launched window: %w", err)
		}
		if err := r.applyFocusMode(opts); err != nil {
			return err
		}
	} else if opts.ShowSpecialAfterLaunch {
		if err := r.revealSpecialWorkspace(opts, targetSpecialWorkspaceName(*started, opts), monitors); err != nil {
			return err
		}
		if err := r.dispatchOne(opts, FormatFocus(started.Address)); err != nil {
			return fmt.Errorf("focusing launched special-workspace window: %w", err)
		}
		if err := r.applyFocusMode(opts); err != nil {
			return err
		}
	}

	r.notify(opts, "hyprsummon", fmt.Sprintf("started %s", opts.Class))
	return nil
}

func (r Runner) focusExisting(client hyprland.Client, activeWorkspaceID int, monitors []hyprland.Monitor, opts Options) error {
	if opts.MoveToSpecial && opts.SpecialWorkspace != "" && !workspaceMatchesTargetSpecial(client.Workspace.Name, opts.SpecialWorkspace) {
		if err := r.dispatchOne(opts, FormatMoveToSpecialSilent(opts.SpecialWorkspace, client.Address)); err != nil {
			return fmt.Errorf("moving window %s to special workspace %s: %w", client.Address, opts.SpecialWorkspace, err)
		}
		client.Workspace = hyprland.WorkspaceType{Id: client.Workspace.Id, Name: "special:" + opts.SpecialWorkspace}
	}

	if opts.ToggleSpecial && workspaceMatchesTargetSpecial(client.Workspace.Name, opts.SpecialWorkspace) {
		if err := r.revealSpecialWorkspace(opts, targetSpecialWorkspaceName(client, opts), monitors); err != nil {
			return err
		}
	}

	if opts.Pull && client.Workspace.Id != activeWorkspaceID {
		if err := r.dispatchOne(opts, FormatPull(activeWorkspaceID, client.Address)); err != nil {
			return fmt.Errorf("pulling window %s: %w", client.Address, err)
		}
	}

	if err := r.dispatchOne(opts, FormatFocus(client.Address)); err != nil {
		return fmt.Errorf("focusing window %s: %w", client.Address, err)
	}
	if err := r.applyFocusMode(opts); err != nil {
		return err
	}

	action := "focused"
	if opts.Pull && client.Workspace.Id != activeWorkspaceID {
		action = "pulled and focused"
	}
	r.notify(opts, "hyprsummon", fmt.Sprintf("%s %s", action, client.Class))
	return nil
}

func (r Runner) launch(opts Options) error {
	dispatch := FormatLaunch(opts)
	r.debugf(opts, "dispatching %q", dispatch)
	if err := r.dispatchOne(opts, dispatch); err != nil {
		return fmt.Errorf("launching %s: %w", opts.Class, err)
	}
	return nil
}

func (r Runner) waitForNewMatch(ctx context.Context, before map[string]struct{}, activeWorkspaceID int, opts Options) (*hyprland.Client, error) {
	deadline := r.Now().Add(pollTimeout)
	for r.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		clients, err := r.Client.Clients()
		if err != nil {
			return nil, fmt.Errorf("listing clients while waiting for launch: %w", err)
		}

		matches := SelectCandidates(FilterClients(clients, opts), activeWorkspaceID, opts)
		for _, client := range matches {
			if _, ok := before[client.Address]; !ok {
				return &client, nil
			}
		}
		if len(matches) > 0 {
			return &matches[0], nil
		}

		r.Sleep(pollInterval)
	}

	return nil, errors.New("timed out waiting for matching window")
}

func (r Runner) dispatchOne(opts Options, param string) error {
	r.debugf(opts, "dispatch %s", param)
	if _, err := r.Client.Dispatch(param); err != nil {
		return err
	}
	return nil
}

func (r Runner) revealSpecialWorkspace(opts Options, workspace string, monitors []hyprland.Monitor) error {
	if specialWorkspaceVisible(monitors, workspace) {
		return nil
	}

	if err := r.dispatchOne(opts, FormatToggleSpecial(workspace)); err != nil {
		return fmt.Errorf("revealing special workspace %s: %w", workspace, err)
	}
	return nil
}

func (r Runner) hideSpecialWorkspace(opts Options, workspace string) error {
	if err := r.dispatchOne(opts, FormatToggleSpecial(workspace)); err != nil {
		return fmt.Errorf("hiding special workspace %s: %w", workspace, err)
	}
	return nil
}

func (r Runner) applyFocusMode(opts Options) error {
	if !opts.Fullscreen && !opts.Maximize {
		return nil
	}

	mode := 0
	if opts.Maximize {
		mode = 1
	}

	if err := r.dispatchOne(opts, FormatFullscreen(mode)); err != nil {
		return fmt.Errorf("applying focus mode: %w", err)
	}

	return nil
}

func (r Runner) notify(opts Options, summary string, body string) {
	if !opts.Notify || r.Notifier == nil {
		return
	}

	if err := r.Notifier.Notify(summary, body); err != nil {
		r.debugf(opts, "notification failed: %v", err)
	}
}

func (r Runner) debugf(opts Options, format string, args ...any) {
	if opts.Debug && r.Notifier != nil {
		if err := r.Notifier.Notify("hyprsummon debug", fmt.Sprintf(format, args...)); err == nil {
			if !opts.Verbose {
				return
			}
		}
	}

	r.verbosef(opts, format, args...)
}

func (r Runner) verbosef(opts Options, format string, args ...any) {
	if !opts.Verbose {
		return
	}
	if r.StdErr == nil {
		return
	}
	fmt.Fprintf(r.StdErr, format+"\n", args...)
}

func FilterClients(clients []hyprland.Client, opts Options) []hyprland.Client {
	filtered := make([]hyprland.Client, 0, len(clients))
	for _, client := range clients {
		if !client.Mapped {
			continue
		}
		if client.Class != opts.Class {
			continue
		}
		if opts.Title != "" && client.Title != opts.Title {
			continue
		}
		if opts.InitialClass != "" && client.InitialClass != opts.InitialClass {
			continue
		}
		filtered = append(filtered, client)
	}

	return filtered
}

func SelectCandidates(matches []hyprland.Client, activeWorkspaceID int, opts Options) []hyprland.Client {
	if len(matches) == 0 {
		return nil
	}

	ordered := make([]hyprland.Client, len(matches))
	copy(ordered, matches)

	if opts.CurrentWorkspaceOnly {
		ordered = filterByWorkspace(ordered, activeWorkspaceID)
	}
	if len(ordered) == 0 {
		return nil
	}

	if opts.ExcludeSpecial {
		ordered = excludeSpecial(ordered)
	}
	if len(ordered) == 0 {
		return nil
	}

	if opts.PreferSpecial {
		ordered = preferSpecial(ordered)
	}
	if opts.PreferFloating {
		ordered = preferFloating(ordered)
	}
	if opts.PreferTiled {
		ordered = preferTiled(ordered)
	}
	if !opts.PreferSpecial && !opts.PreferFloating && !opts.PreferTiled && !opts.CurrentWorkspaceOnly {
		ordered = prioritizeWorkspace(ordered, activeWorkspaceID)
	}

	return ordered
}

func SelectClient(matches []hyprland.Client, activeAddress string, cycle bool) *hyprland.Client {
	if len(matches) == 0 {
		return nil
	}

	ordered := matches
	if !cycle || len(ordered) == 1 {
		return &ordered[0]
	}

	if activeAddress == "" {
		return &ordered[0]
	}

	index := slices.IndexFunc(ordered, func(client hyprland.Client) bool {
		return client.Address == activeAddress
	})
	if index < 0 {
		return &ordered[0]
	}

	next := (index + 1) % len(ordered)
	return &ordered[next]
}

func filterByWorkspace(matches []hyprland.Client, workspaceID int) []hyprland.Client {
	filtered := make([]hyprland.Client, 0, len(matches))
	for _, client := range matches {
		if client.Workspace.Id == workspaceID {
			filtered = append(filtered, client)
		}
	}
	return filtered
}

func prioritizeWorkspace(matches []hyprland.Client, activeWorkspaceID int) []hyprland.Client {
	ordered := make([]hyprland.Client, 0, len(matches))
	for _, client := range matches {
		if client.Workspace.Id == activeWorkspaceID {
			ordered = append(ordered, client)
		}
	}
	for _, client := range matches {
		if client.Workspace.Id != activeWorkspaceID {
			ordered = append(ordered, client)
		}
	}
	return ordered
}

func excludeSpecial(matches []hyprland.Client) []hyprland.Client {
	filtered := make([]hyprland.Client, 0, len(matches))
	for _, client := range matches {
		if !isSpecialWorkspace(client.Workspace.Name) {
			filtered = append(filtered, client)
		}
	}
	return filtered
}

func preferSpecial(matches []hyprland.Client) []hyprland.Client {
	return stablePartition(matches, func(client hyprland.Client) bool {
		return isSpecialWorkspace(client.Workspace.Name)
	})
}

func preferFloating(matches []hyprland.Client) []hyprland.Client {
	return stablePartition(matches, func(client hyprland.Client) bool {
		return client.Floating
	})
}

func preferTiled(matches []hyprland.Client) []hyprland.Client {
	return stablePartition(matches, func(client hyprland.Client) bool {
		return !client.Floating
	})
}

func stablePartition(matches []hyprland.Client, first func(hyprland.Client) bool) []hyprland.Client {
	ordered := make([]hyprland.Client, 0, len(matches))
	for _, client := range matches {
		if first(client) {
			ordered = append(ordered, client)
		}
	}
	for _, client := range matches {
		if !first(client) {
			ordered = append(ordered, client)
		}
	}
	return ordered
}

func FormatFocus(address string) string {
	return fmt.Sprintf("focuswindow address:%s", address)
}

func FormatPull(workspaceID int, address string) string {
	return fmt.Sprintf("movetoworkspace %d,address:%s", workspaceID, address)
}

func FormatMoveToSpecialSilent(workspace string, address string) string {
	return fmt.Sprintf("movetoworkspacesilent special:%s,address:%s", workspace, address)
}

func FormatLaunch(opts Options) string {
	command := JoinCommand(opts.Launch)
	if opts.SpecialWorkspace == "" {
		return "exec " + command
	}

	return fmt.Sprintf("exec [workspace special:%s silent] %s", opts.SpecialWorkspace, command)
}

func FormatToggleSpecial(workspace string) string {
	arg := strings.TrimPrefix(workspace, "special:")
	if arg == "special" {
		arg = ""
	}
	if arg == "" {
		return "togglespecialworkspace"
	}
	return "togglespecialworkspace " + arg
}

func FormatFullscreen(mode int) string {
	return fmt.Sprintf("fullscreen %d set", mode)
}

func JoinCommand(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
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

func isSpecialWorkspace(name string) bool {
	return name == "special" || strings.HasPrefix(name, "special:")
}

func workspaceMatchesTargetSpecial(name string, target string) bool {
	if target == "" {
		return isSpecialWorkspace(name)
	}
	return strings.TrimPrefix(name, "special:") == target && isSpecialWorkspace(name)
}

func targetSpecialWorkspaceName(client hyprland.Client, opts Options) string {
	if opts.SpecialWorkspace != "" {
		return "special:" + opts.SpecialWorkspace
	}
	return client.Workspace.Name
}

func (r Runner) shouldHideSpecial(client hyprland.Client, monitors []hyprland.Monitor, opts Options) bool {
	if !opts.ToggleSpecial {
		return false
	}
	workspace := targetSpecialWorkspaceName(client, opts)
	if !workspaceMatchesTargetSpecial(client.Workspace.Name, opts.SpecialWorkspace) {
		return false
	}
	return specialWorkspaceVisible(monitors, workspace)
}

func specialWorkspaceVisible(monitors []hyprland.Monitor, workspace string) bool {
	for _, monitor := range monitors {
		if monitor.SpecialWorkspace.Name == workspace {
			return true
		}
	}
	return false
}
