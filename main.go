package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"syncoboard-tui/api"
	"syncoboard-tui/ui"
)

func main() {
	cfg := ui.LoadConfig()
	if cfg.Token != "" {
		api.AuthToken = cfg.Token
	}

	p := tea.NewProgram(ui.InitialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
