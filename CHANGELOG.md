# Changelog

All notable changes to hyprsummon are documented here.

## v0.1.0 - 2026-09-04

### Added

- Run-or-raise and focus workflows for Hyprland 0.55 and newer.
- Workspace selection, cycling, pull, fullscreen, maximize, notifications, and scratch-workspace behavior.
- Interactive `hyprsummon setup` with live window discovery, refresh, and manual class entry.
- Hyprland Lua keybind generation using an absolute hyprsummon executable path.
- Non-mutating Lua syntax validation through `hyprctl repl`.
- Accessible, no-color prompts and safe optional file output.
- `wizard` as a backward-compatible alias for `setup`.

### Safety

- Setup never edits or reloads the active Hyprland configuration.
- Existing output files are preserved unless `--force` is explicitly supplied.
