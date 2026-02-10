package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func renderStatus(width int, model string) string {
	statusStyle := lipgloss.NewStyle().
		Width(width).
		Background(lipgloss.Color("#1F2937")).
		Foreground(lipgloss.Color("#9CA3AF")).
		Padding(0, 1)
	return statusStyle.Render(fmt.Sprintf(" GOLEM │ %s │ %s ", model, time.Now().Format("15:04")))
}
