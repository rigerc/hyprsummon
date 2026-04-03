# hyprsummon

> Warning: this is a vibe coded project built by Codex. Treat the code and behavior as experimental, and verify changes before relying on it.

`hyprsummon` is a small Go CLI for Hyprland that implements a practical run-or-raise workflow.

## ✨ At A Glance

- if a matching window already exists, focus it
- otherwise, launch the application

The CLI has three commands:

- `run`: focus a matching window, or launch when no match exists
- `focus`: focus a matching window only
- `wizard`: interactively generate a `hyprsummon` command and optional Hyprland bind

There is no config file. Behavior is controlled entirely through flags.

## 📦 Installation

Quick install:

```bash
go install github.com/rigerc/hyprsummon@latest
```

Then verify it is available:

```bash
hyprsummon --help
```

Install from source:

```bash
git clone https://github.com/rigerc/hyprsummon.git
cd hyprsummon
GOCACHE=/tmp/go-build-cache go install .
```

Build a local binary:

```bash
git clone https://github.com/rigerc/hyprsummon.git
cd hyprsummon
GOCACHE=/tmp/go-build-cache go build -o hyprsummon .
```

Make sure your Go bin directory is on `PATH`. On most systems that is either:

```text
$HOME/go/bin
```

or:

```text
$(go env GOPATH)/bin
```

## 🧭 Overview

`hyprsummon` connects to the current Hyprland instance, lists clients, filters them by exact match rules, selects one candidate, and then either focuses it or launches a new process.

Matching is exact string matching, not regex matching.

- `--class` is required
- `--title` is optional
- `--initial-class` is optional

A client matches only if:

- it is mapped
- its `class` exactly equals `--class`
- if `--title` is set, its `title` exactly equals `--title`
- if `--initial-class` is set, its `initialClass` exactly equals `--initial-class`

## 🎯 Selection And Actions

When multiple windows match:

- by default, `hyprsummon` prefers a match on the active workspace
- if no match is on the active workspace, it falls back to the first remaining match
- `--cycle` walks the ordered match list instead of always choosing the first one

Useful selection flags:

- `--current-workspace-only`
- `--all-workspaces`
- `--prefer-floating`
- `--prefer-tiled`
- `--prefer-special`
- `--exclude-special`

Existing-window actions:

- default: focus the selected match
- `--pull`: move the selected match to the active workspace, then focus it
- `--move-to-special --special-workspace NAME`: move the selected match to `special:NAME`
- `--toggle-special --special-workspace NAME`: reveal a hidden target special workspace before focus, or hide it when the selected window is already focused there
- `--fullscreen` or `--maximize`: apply a focus mode after selection

Launch actions:

- default: launch and briefly poll for a matching new window, then focus it
- `--background`: launch and return immediately
- `--special-workspace NAME`: launch onto `special:NAME`
- `--show-special-after-launch`: reveal the target special workspace after launch, then focus the new matching window

## 🪄 Scratch Mode

If an app should live on a named special workspace, use `--scratch` with `--special-workspace NAME`.

`--scratch` is the high-level alias for the common scratchpad workflow.

For `run`, it expands to:

```text
--prefer-special --move-to-special --toggle-special --show-special-after-launch
```

For `focus`, it expands to:

```text
--prefer-special --move-to-special --toggle-special
```

This is the intended mode for app-specific special-workspace toggles such as music players, scratch terminals, or utility apps.

## 💡 Examples

Basic run-or-raise:

```bash
hyprsummon run --class kitty -- kitty
```

Focus only:

```bash
hyprsummon focus --class kitty
```

Bring an existing app to the current workspace:

```bash
hyprsummon run --class firefox --pull -- firefox
```

Restrict matching to the current workspace:

```bash
hyprsummon run --class org.gnome.Nautilus --current-workspace-only -- nautilus
```

Named special-workspace scratch toggle:

```bash
hyprsummon run --class spotify --special-workspace music --scratch -- spotify-launcher
```

Launch directly onto a named special workspace without scratch semantics:

```bash
hyprsummon run --class kitty --special-workspace scratch -- kitty --class scratchpad
```

Show diagnostics on stderr:

```bash
hyprsummon run --class kitty --verbose -- kitty
```

## 📚 Help

Use built-in help for current flag details:

```bash
hyprsummon --help
hyprsummon wizard
hyprsummon run --help
hyprsummon focus --help
hyprsummon help flag --class
hyprsummon help flag --toggle-special
```

## 🧙 Wizard

Use the wizard when you want `hyprsummon` to generate the command for you.

```bash
hyprsummon wizard
```

The wizard is built with `huh` and walks through:

- intent selection
- class/title matching
- launch command setup
- special-workspace or scratch behavior
- optional advanced flags
- optional Hyprland bind generation

## 🛠️ Build And Test

Build:

```bash
cd /mnt/extra-ssd/dev/projects/hyprsummon
GOCACHE=/tmp/go-build-cache go build -o hyprsummon .
```

Run without installing:

```bash
cd /mnt/extra-ssd/dev/projects/hyprsummon
GOCACHE=/tmp/go-build-cache go run . run --class kitty -- kitty
```

Test:

```bash
cd /mnt/extra-ssd/dev/projects/hyprsummon
GOCACHE=/tmp/go-build-cache go test ./...
```

## ✅ Requirements

- Linux
- Go 1.26+
- Hyprland running in the current session
- access to the Hyprland IPC socket
- `XDG_RUNTIME_DIR` set

Optional:

- `notify-send` for `--notify` and `--debug`

## 🔌 Hyprland IPC

`hyprsummon` connects to:

```text
$XDG_RUNTIME_DIR/hypr/$HYPRLAND_INSTANCE_SIGNATURE/.socket.sock
```

If `HYPRLAND_INSTANCE_SIGNATURE` is missing, it falls back to discovering a Hyprland socket under `$XDG_RUNTIME_DIR/hypr`.

## ⚠️ Limitations

- no config file support
- no regex matching
- no persistent cycle state across invocations
- no fallback notification backend beyond `notify-send`
- special-workspace behavior is tied to explicit flags; there is no automatic policy layer beyond `--scratch`
