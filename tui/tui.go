package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	modeSelector = iota
	modeMiner
	modeUser
	modeQuery
)

// Main model
type model struct {
	mode          int
	selectedIndex int
	options       []string
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	tea.Batch()
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			return m, tea.Quit
		case "down", "j":
			if m.selectedIndex < len(m.options)-1 {
				m.selectedIndex++
			}
		case "up", "k":
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
		case "enter":
			if m.selectedIndex == 0 {
				m.mode = modeMiner
			} else if m.selectedIndex == 1 {
				m.mode = modeUser
				usrWallet := InitialUsrWallet()
				return usrWallet.Update(msg)
			} else if m.selectedIndex == 2 {

			}
		}

	case tea.WindowSizeMsg:
		winWidth = msg.Width
		winHeight = msg.Height
	}

	return m, nil
}

func (m model) View() string {
	var content string

	if m.mode == modeSelector {
		content = "~~ Select an option ~~\n\n"
		for i, option := range m.options {
			if i == m.selectedIndex {
				content += "=> " + selectedStyle.Render(option) + "\n\n"
			} else {
				content += "=> " + unSelectedStyle.Render(option) + "\n\n"
			}
		}
		content += helpStyle.Render("\nPress Esc to quit.")
	}

	return Centered(m, content, winWidth, winHeight)
}

func Cli() {
	m := model{
		mode:    modeSelector,
		options: []string{"Start a miner", "Create a user wallet", "Query the blockchain"},
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}
