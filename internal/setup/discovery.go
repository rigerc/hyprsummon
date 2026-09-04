package setup

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	hyprland "github.com/thiagokokada/hyprland-go"
)

type AppChoice struct {
	Class        string
	InitialClass string
	Title        string
	Workspace    string
	Count        int
	Active       bool
}

func DiscoverApps(clients []hyprland.Client, activeAddress string) []AppChoice {
	byClass := make(map[string]AppChoice, len(clients))
	for _, client := range clients {
		if !client.Mapped || strings.TrimSpace(client.Class) == "" {
			continue
		}
		choice, found := byClass[client.Class]
		isActive := client.Address == activeAddress
		if !found || isActive {
			choice.Class = client.Class
			choice.InitialClass = client.InitialClass
			choice.Title = client.Title
			choice.Workspace = client.Workspace.Name
		}
		choice.Count++
		choice.Active = choice.Active || isActive
		byClass[client.Class] = choice
	}

	choices := make([]AppChoice, 0, len(byClass))
	for _, choice := range byClass {
		choices = append(choices, choice)
	}
	sort.Slice(choices, func(i, j int) bool { return choices[i].Class < choices[j].Class })
	return choices
}

func (a AppChoice) Label() string {
	class := sanitizeTerminalText(a.Class)
	initialClass := sanitizeTerminalText(a.InitialClass)
	title := sanitizeTerminalText(a.Title)
	workspace := sanitizeTerminalText(a.Workspace)
	details := make([]string, 0, 4)
	if title != "" {
		details = append(details, title)
	}
	if initialClass != "" && initialClass != class {
		details = append(details, "initial: "+initialClass)
	}
	if workspace != "" {
		details = append(details, "workspace: "+workspace)
	}
	details = append(details, fmt.Sprintf("%d window(s)", a.Count))
	if a.Active {
		details = append(details, "active")
	}
	return class + " — " + strings.Join(details, " • ")
}

func sanitizeTerminalText(value string) string {
	return strings.Join(strings.FieldsFunc(value, func(r rune) bool {
		return unicode.IsControl(r) || unicode.In(r, unicode.Cf)
	}), " ")
}
