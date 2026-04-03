package hypr

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	hyprland "github.com/thiagokokada/hyprland-go"
)

func TestFilterClients(t *testing.T) {
	opts := Options{
		Class:        "kitty",
		Title:        "scratch",
		InitialClass: "kitty",
	}
	clients := []hyprland.Client{
		{Address: "0x1", Mapped: true, Class: "kitty", Title: "scratch", InitialClass: "kitty"},
		{Address: "0x2", Mapped: false, Class: "kitty", Title: "scratch", InitialClass: "kitty"},
		{Address: "0x3", Mapped: true, Class: "kitty", Title: "other", InitialClass: "kitty"},
		{Address: "0x4", Mapped: true, Class: "foot", Title: "scratch", InitialClass: "foot"},
	}

	got := FilterClients(clients, opts)
	if len(got) != 1 || got[0].Address != "0x1" {
		t.Fatalf("FilterClients() = %#v, want only 0x1", got)
	}
}

func TestSelectCandidatesCurrentWorkspacePreferred(t *testing.T) {
	matches := []hyprland.Client{
		{Address: "0x1", Workspace: hyprland.WorkspaceType{Id: 2}},
		{Address: "0x2", Workspace: hyprland.WorkspaceType{Id: 1}},
		{Address: "0x3", Workspace: hyprland.WorkspaceType{Id: 3}},
	}

	got := SelectCandidates(matches, 1, Options{})
	if len(got) == 0 || got[0].Address != "0x2" {
		t.Fatalf("SelectCandidates() = %#v, want address 0x2 first", got)
	}
}

func TestSelectClientCycleWrapsFromActive(t *testing.T) {
	matches := []hyprland.Client{
		{Address: "0x1", Workspace: hyprland.WorkspaceType{Id: 1}},
		{Address: "0x2", Workspace: hyprland.WorkspaceType{Id: 1}},
		{Address: "0x3", Workspace: hyprland.WorkspaceType{Id: 2}},
	}

	got := SelectClient(matches, "0x2", true)
	if got == nil || got.Address != "0x3" {
		t.Fatalf("SelectClient() = %#v, want address 0x3", got)
	}
}

func TestSelectCandidatesCurrentWorkspaceOnly(t *testing.T) {
	matches := []hyprland.Client{
		{Address: "0x1", Workspace: hyprland.WorkspaceType{Id: 2}},
		{Address: "0x2", Workspace: hyprland.WorkspaceType{Id: 1}},
	}

	got := SelectCandidates(matches, 1, Options{CurrentWorkspaceOnly: true})
	want := []hyprland.Client{{Address: "0x2", Workspace: hyprland.WorkspaceType{Id: 1}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SelectCandidates() = %#v, want %#v", got, want)
	}
}

func TestSelectCandidatesPreferFloating(t *testing.T) {
	matches := []hyprland.Client{
		{Address: "0x1", Floating: false, Workspace: hyprland.WorkspaceType{Id: 1}},
		{Address: "0x2", Floating: true, Workspace: hyprland.WorkspaceType{Id: 2}},
	}

	got := SelectCandidates(matches, 1, Options{PreferFloating: true})
	if len(got) == 0 || got[0].Address != "0x2" {
		t.Fatalf("SelectCandidates() = %#v, want floating window first", got)
	}
}

func TestSelectCandidatesExcludeSpecial(t *testing.T) {
	matches := []hyprland.Client{
		{Address: "0x1", Workspace: hyprland.WorkspaceType{Id: -99, Name: "special:scratch"}},
		{Address: "0x2", Workspace: hyprland.WorkspaceType{Id: 1, Name: "1"}},
	}

	got := SelectCandidates(matches, 1, Options{ExcludeSpecial: true})
	want := []hyprland.Client{{Address: "0x2", Workspace: hyprland.WorkspaceType{Id: 1, Name: "1"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SelectCandidates() = %#v, want %#v", got, want)
	}
}

func TestSelectCandidatesPreferSpecial(t *testing.T) {
	matches := []hyprland.Client{
		{Address: "0x1", Workspace: hyprland.WorkspaceType{Id: 1, Name: "1"}},
		{Address: "0x2", Workspace: hyprland.WorkspaceType{Id: -99, Name: "special:scratch"}},
	}

	got := SelectCandidates(matches, 1, Options{PreferSpecial: true})
	if len(got) == 0 || got[0].Address != "0x2" {
		t.Fatalf("SelectCandidates() = %#v, want special window first", got)
	}
}

func TestFormatLaunch(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want string
	}{
		{
			name: "plain",
			opts: Options{Launch: []string{"kitty", "--class", "scratch"}},
			want: "exec kitty --class scratch",
		},
		{
			name: "special workspace",
			opts: Options{
				SpecialWorkspace: "scratch",
				Launch:           []string{"kitty", "--class", "scratch pad"},
			},
			want: "exec [workspace special:scratch silent] kitty --class 'scratch pad'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatLaunch(tt.opts); got != tt.want {
				t.Errorf("FormatLaunch() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatPull(t *testing.T) {
	got := FormatPull(7, "0xabc")
	want := "movetoworkspace 7,address:0xabc"
	if got != want {
		t.Fatalf("FormatPull() = %q, want %q", got, want)
	}
}

func TestFormatMoveToSpecialSilent(t *testing.T) {
	got := FormatMoveToSpecialSilent("music", "0xabc")
	want := "movetoworkspacesilent special:music,address:0xabc"
	if got != want {
		t.Fatalf("FormatMoveToSpecialSilent() = %q, want %q", got, want)
	}
}

func TestFormatToggleSpecial(t *testing.T) {
	if got := FormatToggleSpecial("special:scratch"); got != "togglespecialworkspace scratch" {
		t.Fatalf("FormatToggleSpecial() = %q, want togglespecialworkspace scratch", got)
	}
}

func TestFormatFullscreen(t *testing.T) {
	if got := FormatFullscreen(1); got != "fullscreen 1 set" {
		t.Fatalf("FormatFullscreen() = %q, want fullscreen 1 set", got)
	}
}

func TestRunnerRunExistingWindowFocused(t *testing.T) {
	client := &fakeClient{
		clients: []hyprland.Client{
			{Address: "0x1", Mapped: true, Class: "kitty", Workspace: hyprland.WorkspaceType{Id: 3}},
		},
		activeWindow:    hyprland.Window{Client: hyprland.Client{Address: "0x9"}},
		activeWorkspace: hyprland.Workspace{WorkspaceType: hyprland.WorkspaceType{Id: 3}},
	}
	notifier := &fakeNotifier{}
	runner := Runner{
		Client:   client,
		Notifier: notifier,
		StdErr:   io.Discard,
		Now:      fakeNow(),
		Sleep:    func(time.Duration) {},
	}

	err := runner.Run(context.Background(), Options{Class: "kitty", Notify: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	wantDispatches := []string{"focuswindow address:0x1"}
	if !reflect.DeepEqual(client.dispatches, wantDispatches) {
		t.Fatalf("dispatches = %#v, want %#v", client.dispatches, wantDispatches)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("notifications = %d, want 1", len(notifier.calls))
	}
}

func TestRunnerRunPullsExistingWindow(t *testing.T) {
	client := &fakeClient{
		clients: []hyprland.Client{
			{Address: "0x1", Mapped: true, Class: "kitty", Workspace: hyprland.WorkspaceType{Id: 2}},
		},
		activeWindow:    hyprland.Window{Client: hyprland.Client{Address: "0x9"}},
		activeWorkspace: hyprland.Workspace{WorkspaceType: hyprland.WorkspaceType{Id: 4}},
	}
	runner := Runner{
		Client: client,
		StdErr: io.Discard,
		Now:    fakeNow(),
		Sleep:  func(time.Duration) {},
	}

	err := runner.Run(context.Background(), Options{Class: "kitty", Pull: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	wantDispatches := []string{
		"movetoworkspace 4,address:0x1",
		"focuswindow address:0x1",
	}
	if !reflect.DeepEqual(client.dispatches, wantDispatches) {
		t.Fatalf("dispatches = %#v, want %#v", client.dispatches, wantDispatches)
	}
}

func TestRunnerRunTogglesHiddenSpecialBeforeFocus(t *testing.T) {
	client := &fakeClient{
		clients: []hyprland.Client{
			{Address: "0x1", Mapped: true, Class: "kitty", Workspace: hyprland.WorkspaceType{Id: -99, Name: "special:scratch"}},
		},
		activeWindow:    hyprland.Window{Client: hyprland.Client{Address: "0x9"}},
		activeWorkspace: hyprland.Workspace{WorkspaceType: hyprland.WorkspaceType{Id: 4}},
		monitors:        []hyprland.Monitor{{SpecialWorkspace: hyprland.WorkspaceType{Name: ""}}},
	}
	runner := Runner{
		Client: client,
		StdErr: io.Discard,
		Now:    fakeNow(),
		Sleep:  func(time.Duration) {},
	}

	err := runner.Run(context.Background(), Options{Class: "kitty", ToggleSpecial: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	wantDispatches := []string{
		"togglespecialworkspace scratch",
		"focuswindow address:0x1",
	}
	if !reflect.DeepEqual(client.dispatches, wantDispatches) {
		t.Fatalf("dispatches = %#v, want %#v", client.dispatches, wantDispatches)
	}
}

func TestRunnerRunMovesExistingWindowToSpecialThenReveals(t *testing.T) {
	client := &fakeClient{
		clients: []hyprland.Client{
			{Address: "0x1", Mapped: true, Class: "spotify", Workspace: hyprland.WorkspaceType{Id: 2, Name: "2"}},
		},
		activeWindow:    hyprland.Window{Client: hyprland.Client{Address: "0x9"}},
		activeWorkspace: hyprland.Workspace{WorkspaceType: hyprland.WorkspaceType{Id: 4}},
		monitors:        []hyprland.Monitor{{SpecialWorkspace: hyprland.WorkspaceType{Name: ""}}},
	}
	runner := Runner{
		Client: client,
		StdErr: io.Discard,
		Now:    fakeNow(),
		Sleep:  func(time.Duration) {},
	}

	err := runner.Run(context.Background(), Options{
		Class:            "spotify",
		SpecialWorkspace: "music",
		MoveToSpecial:    true,
		ToggleSpecial:    true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	wantDispatches := []string{
		"movetoworkspacesilent special:music,address:0x1",
		"togglespecialworkspace music",
		"focuswindow address:0x1",
	}
	if !reflect.DeepEqual(client.dispatches, wantDispatches) {
		t.Fatalf("dispatches = %#v, want %#v", client.dispatches, wantDispatches)
	}
}

func TestRunnerRunFocusedVisibleSpecialTogglesHidden(t *testing.T) {
	client := &fakeClient{
		clients: []hyprland.Client{
			{Address: "0x1", Mapped: true, Class: "spotify", Workspace: hyprland.WorkspaceType{Id: -99, Name: "special:music"}},
		},
		activeWindow:    hyprland.Window{Client: hyprland.Client{Address: "0x1"}},
		activeWorkspace: hyprland.Workspace{WorkspaceType: hyprland.WorkspaceType{Id: -99, Name: "special:music"}},
		monitors:        []hyprland.Monitor{{SpecialWorkspace: hyprland.WorkspaceType{Name: "special:music"}}},
	}
	runner := Runner{
		Client: client,
		StdErr: io.Discard,
		Now:    fakeNow(),
		Sleep:  func(time.Duration) {},
	}

	err := runner.Run(context.Background(), Options{
		Class:            "spotify",
		SpecialWorkspace: "music",
		ToggleSpecial:    true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	wantDispatches := []string{"togglespecialworkspace music"}
	if !reflect.DeepEqual(client.dispatches, wantDispatches) {
		t.Fatalf("dispatches = %#v, want %#v", client.dispatches, wantDispatches)
	}
}

func TestRunnerRunVisibleSpecialFocusesWithoutToggleWhenNotActive(t *testing.T) {
	client := &fakeClient{
		clients: []hyprland.Client{
			{Address: "0x1", Mapped: true, Class: "spotify", Workspace: hyprland.WorkspaceType{Id: -99, Name: "special:music"}},
		},
		activeWindow:    hyprland.Window{Client: hyprland.Client{Address: "0x9"}},
		activeWorkspace: hyprland.Workspace{WorkspaceType: hyprland.WorkspaceType{Id: 4, Name: "4"}},
		monitors:        []hyprland.Monitor{{SpecialWorkspace: hyprland.WorkspaceType{Name: "special:music"}}},
	}
	runner := Runner{
		Client: client,
		StdErr: io.Discard,
		Now:    fakeNow(),
		Sleep:  func(time.Duration) {},
	}

	err := runner.Run(context.Background(), Options{
		Class:            "spotify",
		SpecialWorkspace: "music",
		ToggleSpecial:    true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	wantDispatches := []string{"focuswindow address:0x1"}
	if !reflect.DeepEqual(client.dispatches, wantDispatches) {
		t.Fatalf("dispatches = %#v, want %#v", client.dispatches, wantDispatches)
	}
}

func TestRunnerRunAppliesMaximizeAfterFocus(t *testing.T) {
	client := &fakeClient{
		clients: []hyprland.Client{
			{Address: "0x1", Mapped: true, Class: "kitty", Workspace: hyprland.WorkspaceType{Id: 3}},
		},
		activeWindow:    hyprland.Window{Client: hyprland.Client{Address: "0x9"}},
		activeWorkspace: hyprland.Workspace{WorkspaceType: hyprland.WorkspaceType{Id: 3}},
	}
	runner := Runner{
		Client: client,
		StdErr: io.Discard,
		Now:    fakeNow(),
		Sleep:  func(time.Duration) {},
	}

	err := runner.Run(context.Background(), Options{Class: "kitty", Maximize: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	wantDispatches := []string{
		"focuswindow address:0x1",
		"fullscreen 1 set",
	}
	if !reflect.DeepEqual(client.dispatches, wantDispatches) {
		t.Fatalf("dispatches = %#v, want %#v", client.dispatches, wantDispatches)
	}
}

func TestRunnerRunLaunchesInBackground(t *testing.T) {
	client := &fakeClient{
		clients:         nil,
		activeWorkspace: hyprland.Workspace{WorkspaceType: hyprland.WorkspaceType{Id: 4}},
		activeWindow:    hyprland.Window{},
		clientSnapshots: [][]hyprland.Client{},
	}
	runner := Runner{
		Client: client,
		StdErr: io.Discard,
		Now:    fakeNow(),
		Sleep:  func(time.Duration) {},
	}

	err := runner.Run(context.Background(), Options{
		Class:      "kitty",
		Background: true,
		Launch:     []string{"kitty"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	wantDispatches := []string{"exec kitty"}
	if !reflect.DeepEqual(client.dispatches, wantDispatches) {
		t.Fatalf("dispatches = %#v, want %#v", client.dispatches, wantDispatches)
	}
}

func TestRunnerRunLaunchesAndFocusesNewWindow(t *testing.T) {
	client := &fakeClient{
		clients:         nil,
		activeWorkspace: hyprland.Workspace{WorkspaceType: hyprland.WorkspaceType{Id: 4}},
		activeWindow:    hyprland.Window{},
		clientSnapshots: [][]hyprland.Client{
			{},
			{
				{Address: "0x2", Mapped: true, Class: "kitty", Workspace: hyprland.WorkspaceType{Id: 4}},
			},
		},
	}
	runner := Runner{
		Client: client,
		StdErr: io.Discard,
		Now:    advancingNow(),
		Sleep:  func(time.Duration) {},
	}

	err := runner.Run(context.Background(), Options{
		Class:  "kitty",
		Launch: []string{"kitty"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	wantDispatches := []string{
		"exec kitty",
		"focuswindow address:0x2",
	}
	if !reflect.DeepEqual(client.dispatches, wantDispatches) {
		t.Fatalf("dispatches = %#v, want %#v", client.dispatches, wantDispatches)
	}
}

func TestRunnerRunLaunchesSpecialWorkspaceWithoutRefocus(t *testing.T) {
	client := &fakeClient{
		activeWorkspace: hyprland.Workspace{WorkspaceType: hyprland.WorkspaceType{Id: 4}},
		activeWindow:    hyprland.Window{},
		clientSnapshots: [][]hyprland.Client{
			{},
			{
				{Address: "0x2", Mapped: true, Class: "kitty", Workspace: hyprland.WorkspaceType{Id: -99}},
			},
		},
	}
	runner := Runner{
		Client: client,
		StdErr: io.Discard,
		Now:    advancingNow(),
		Sleep:  func(time.Duration) {},
	}

	err := runner.Run(context.Background(), Options{
		Class:            "kitty",
		SpecialWorkspace: "scratch",
		Launch:           []string{"kitty"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	wantDispatches := []string{"exec [workspace special:scratch silent] kitty"}
	if !reflect.DeepEqual(client.dispatches, wantDispatches) {
		t.Fatalf("dispatches = %#v, want %#v", client.dispatches, wantDispatches)
	}
}

func TestRunnerRunLaunchesSpecialWorkspaceAndRevealsWhenRequested(t *testing.T) {
	client := &fakeClient{
		activeWorkspace: hyprland.Workspace{WorkspaceType: hyprland.WorkspaceType{Id: 4}},
		activeWindow:    hyprland.Window{},
		monitors:        []hyprland.Monitor{{SpecialWorkspace: hyprland.WorkspaceType{Name: ""}}},
		clientSnapshots: [][]hyprland.Client{
			{},
			{
				{Address: "0x2", Mapped: true, Class: "spotify", Workspace: hyprland.WorkspaceType{Id: -99, Name: "special:music"}},
			},
		},
	}
	runner := Runner{
		Client: client,
		StdErr: io.Discard,
		Now:    advancingNow(),
		Sleep:  func(time.Duration) {},
	}

	err := runner.Run(context.Background(), Options{
		Class:                  "spotify",
		SpecialWorkspace:       "music",
		ShowSpecialAfterLaunch: true,
		Launch:                 []string{"spotify-launcher"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	wantDispatches := []string{
		"exec [workspace special:music silent] spotify-launcher",
		"togglespecialworkspace music",
		"focuswindow address:0x2",
	}
	if !reflect.DeepEqual(client.dispatches, wantDispatches) {
		t.Fatalf("dispatches = %#v, want %#v", client.dispatches, wantDispatches)
	}
}

func TestRunnerRunLaunchesSpecialWorkspaceRevealAppliesMaximizeAfterFocus(t *testing.T) {
	client := &fakeClient{
		activeWorkspace: hyprland.Workspace{WorkspaceType: hyprland.WorkspaceType{Id: 4}},
		activeWindow:    hyprland.Window{},
		monitors:        []hyprland.Monitor{{SpecialWorkspace: hyprland.WorkspaceType{Name: ""}}},
		clientSnapshots: [][]hyprland.Client{
			{},
			{
				{Address: "0x2", Mapped: true, Class: "spotify", Workspace: hyprland.WorkspaceType{Id: -99, Name: "special:music"}},
			},
		},
	}
	runner := Runner{
		Client: client,
		StdErr: io.Discard,
		Now:    advancingNow(),
		Sleep:  func(time.Duration) {},
	}

	err := runner.Run(context.Background(), Options{
		Class:                  "spotify",
		SpecialWorkspace:       "music",
		ShowSpecialAfterLaunch: true,
		Maximize:               true,
		Launch:                 []string{"spotify-launcher"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	wantDispatches := []string{
		"exec [workspace special:music silent] spotify-launcher",
		"togglespecialworkspace music",
		"focuswindow address:0x2",
		"fullscreen 1 set",
	}
	if !reflect.DeepEqual(client.dispatches, wantDispatches) {
		t.Fatalf("dispatches = %#v, want %#v", client.dispatches, wantDispatches)
	}
}

func TestOptionsValidateMoveToSpecialRequiresTarget(t *testing.T) {
	err := (Options{Class: "spotify", MoveToSpecial: true}).Validate()
	if err == nil || err.Error() != "--move-to-special requires --special-workspace" {
		t.Fatalf("Validate() error = %v, want move-to-special requirement", err)
	}
}

func TestOptionsValidateShowSpecialAfterLaunchRequiresTarget(t *testing.T) {
	err := (Options{Class: "spotify", ShowSpecialAfterLaunch: true}).Validate()
	if err == nil || err.Error() != "--show-special-after-launch requires --special-workspace" {
		t.Fatalf("Validate() error = %v, want show-special-after-launch requirement", err)
	}
}

func TestOptionsValidateMoveToSpecialConflictsWithPull(t *testing.T) {
	err := (Options{Class: "spotify", MoveToSpecial: true, Pull: true, SpecialWorkspace: "music"}).Validate()
	if err == nil || err.Error() != "--move-to-special and --pull cannot be used together" {
		t.Fatalf("Validate() error = %v, want move-to-special/pull conflict", err)
	}
}

func TestOptionsValidateScratchRequiresNoExtraValidationByItself(t *testing.T) {
	err := (Options{Class: "spotify", Scratch: true}).Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil before mode expansion", err)
	}
}

func TestRunnerRunIgnoresMissingNotifier(t *testing.T) {
	client := &fakeClient{
		clients: []hyprland.Client{
			{Address: "0x1", Mapped: true, Class: "kitty", Workspace: hyprland.WorkspaceType{Id: 3}},
		},
		activeWindow:    hyprland.Window{Client: hyprland.Client{Address: "0x9"}},
		activeWorkspace: hyprland.Workspace{WorkspaceType: hyprland.WorkspaceType{Id: 3}},
	}
	runner := Runner{
		Client:   client,
		Notifier: fakeNotifierErr{},
		StdErr:   io.Discard,
		Now:      fakeNow(),
		Sleep:    func(time.Duration) {},
	}

	err := runner.Run(context.Background(), Options{Class: "kitty", Notify: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunnerRunDebugSendsNotifications(t *testing.T) {
	client := &fakeClient{
		clients:         nil,
		activeWorkspace: hyprland.Workspace{WorkspaceType: hyprland.WorkspaceType{Id: 4}},
		activeWindow:    hyprland.Window{},
		clientSnapshots: [][]hyprland.Client{
			{},
			{
				{Address: "0x2", Mapped: true, Class: "kitty", Workspace: hyprland.WorkspaceType{Id: 4}},
			},
		},
	}
	notifier := &fakeNotifier{}
	var stderr bytes.Buffer
	runner := Runner{
		Client:   client,
		Notifier: notifier,
		StdErr:   &stderr,
		Now:      advancingNow(),
		Sleep:    func(time.Duration) {},
	}

	err := runner.Run(context.Background(), Options{
		Class:  "kitty",
		Debug:  true,
		Launch: []string{"kitty"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(notifier.calls) == 0 {
		t.Fatal("notifications = 0, want debug notifications")
	}
	if !strings.Contains(notifier.calls[0], "hyprsummon debug: dispatching") {
		t.Fatalf("first notification = %q, want debug dispatch message", notifier.calls[0])
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want no verbose output", stderr.String())
	}
}

func TestRunnerRunDebugAndVerboseBothEmitDiagnostics(t *testing.T) {
	client := &fakeClient{
		clients:         nil,
		activeWorkspace: hyprland.Workspace{WorkspaceType: hyprland.WorkspaceType{Id: 4}},
		activeWindow:    hyprland.Window{},
		clientSnapshots: [][]hyprland.Client{{}},
	}
	notifier := &fakeNotifier{}
	var stderr bytes.Buffer
	runner := Runner{
		Client:   client,
		Notifier: notifier,
		StdErr:   &stderr,
		Now:      advancingNow(),
		Sleep:    func(time.Duration) {},
	}

	err := runner.Run(context.Background(), Options{
		Class:   "kitty",
		Debug:   true,
		Verbose: true,
		Launch:  []string{"kitty"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(notifier.calls) == 0 {
		t.Fatal("notifications = 0, want debug notifications")
	}
	if !strings.Contains(stderr.String(), "dispatching \"exec kitty\"") {
		t.Fatalf("stderr = %q, want debug diagnostics", stderr.String())
	}
}

type fakeClient struct {
	clients         []hyprland.Client
	clientSnapshots [][]hyprland.Client
	monitors        []hyprland.Monitor
	activeWindow    hyprland.Window
	activeWorkspace hyprland.Workspace
	dispatches      []string
	clientCalls     int
}

func (f *fakeClient) Clients() ([]hyprland.Client, error) {
	if f.clientCalls < len(f.clientSnapshots) {
		clients := f.clientSnapshots[f.clientCalls]
		f.clientCalls++
		return clients, nil
	}
	return f.clients, nil
}

func (f *fakeClient) ActiveWindow() (hyprland.Window, error) {
	return f.activeWindow, nil
}

func (f *fakeClient) ActiveWorkspace() (hyprland.Workspace, error) {
	return f.activeWorkspace, nil
}

func (f *fakeClient) Monitors() ([]hyprland.Monitor, error) {
	return f.monitors, nil
}

func (f *fakeClient) Dispatch(params ...string) ([]hyprland.Response, error) {
	f.dispatches = append(f.dispatches, params...)
	return nil, nil
}

type fakeNotifier struct {
	calls []string
}

func (f *fakeNotifier) Notify(summary string, body string) error {
	f.calls = append(f.calls, summary+": "+body)
	return nil
}

type fakeNotifierErr struct{}

func (fakeNotifierErr) Notify(string, string) error {
	return errors.New("notify failed")
}

func fakeNow() func() time.Time {
	now := time.Unix(0, 0)
	return func() time.Time {
		return now
	}
}

func advancingNow() func() time.Time {
	now := time.Unix(0, 0)
	return func() time.Time {
		now = now.Add(200 * time.Millisecond)
		return now
	}
}
