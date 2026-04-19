package cli

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// jogaiTheme returns the huh form theme used by interactive wizards.
// Palette: slate grays with a deep-blue accent — sober, avoids generic AI neon.
func jogaiTheme() *huh.Theme {
	t := huh.ThemeBase()

	var (
		primary     = lipgloss.AdaptiveColor{Light: "#9a3412", Dark: "#fdba74"}
		accent      = lipgloss.AdaptiveColor{Light: "#c2410c", Dark: "#fb923c"}
		description = lipgloss.AdaptiveColor{Light: "#64748b", Dark: "#94a3b8"}
		muted       = lipgloss.AdaptiveColor{Light: "#94a3b8", Dark: "#64748b"}
		errColor    = lipgloss.AdaptiveColor{Light: "#dc2626", Dark: "#f87171"}
		border      = lipgloss.AdaptiveColor{Light: "#cbd5e1", Dark: "#334155"}
	)

	t.Focused.Base = t.Focused.Base.BorderForeground(primary)
	t.Focused.Card = t.Focused.Base
	t.Focused.Title = t.Focused.Title.Foreground(primary).Bold(true)
	t.Focused.NoteTitle = t.Focused.NoteTitle.Foreground(primary).Bold(true).MarginBottom(1)
	t.Focused.Description = t.Focused.Description.Foreground(description)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(errColor)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(errColor)

	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(accent)
	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(accent)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(muted)

	t.Blurred = t.Focused
	t.Blurred.Base = t.Blurred.Base.BorderStyle(lipgloss.HiddenBorder()).BorderForeground(border)
	t.Blurred.Card = t.Blurred.Base
	t.Blurred.Title = t.Blurred.Title.Foreground(muted).Bold(false)
	t.Blurred.Description = t.Blurred.Description.Foreground(muted)

	t.Group.Title = t.Focused.Title
	t.Group.Description = t.Focused.Description

	return t
}
