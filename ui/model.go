package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	virtualPath    string
	selectedTaskId string
	activeBoardId  string

	textInput textinput.Model
	viewport  viewport.Model

	outputHistory  []string
	commandHistory []string
	historyIndex   int

	width  int
	height int

	// UI State for layout
	workspaces  []string
	tasks       []string
	taskDetails string

	err error
}

func InitialModel() Model {
	ti := textinput.New()
	ti.Placeholder = "Enter command..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50

	return Model{
		virtualPath:    "/",
		textInput:      ti,
		outputHistory:  []string{"Welcome to Syncoboard TUI!", "Type /auth to login.", "Type /help to see commands."},
		commandHistory: []string{},
		historyIndex:   -1,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		fetchWorkspaces(),
		fetchTasks(m.virtualPath),
		updateLastOnlineCmd(),
	)
}

var (
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")).
			Underline(true)

	promptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	pathStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
)

func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	// Calculate panel widths
	panelWidth := (m.width - 6) / 3

	// Left: Workspaces
	wsContent := titleStyle.Render("Workspaces") + "\n"
	if len(m.workspaces) == 0 {
		wsContent += lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("No workspaces loaded.\nRun /cd to navigate.")
	} else {
		wsContent += strings.Join(m.workspaces, "\n")
	}
	wsPanel := panelStyle.Copy().Width(panelWidth).Height(10).Render(wsContent)

	// Middle: Tasks
	taskContent := titleStyle.Render("Tasks") + "\n"
	if len(m.tasks) == 0 {
		taskContent += lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("No tasks loaded.")
	} else {
		taskContent += strings.Join(m.tasks, "\n")
	}
	taskPanel := panelStyle.Copy().Width(panelWidth).Height(10).Render(taskContent)

	// Right: Task Details
	detailsContent := titleStyle.Render("Task Details") + "\n"
	if m.taskDetails == "" {
		detailsContent += lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Select a task to view details.")
	} else {
		detailsContent += m.taskDetails
	}
	detailsPanel := panelStyle.Copy().Width(panelWidth).Height(10).Render(detailsContent)

	topSection := lipgloss.JoinHorizontal(lipgloss.Top, wsPanel, taskPanel, detailsPanel)

	// Output area
	outputBox := panelStyle.Copy().
		Width(m.width - 2).
		Height(m.viewport.Height + 2).
		Render(m.viewport.View())

	// Input area
	prompt := pathStyle.Render(m.virtualPath+" ") + promptStyle.Render("$ ")
	inputLine := prompt + m.textInput.View()

	return lipgloss.JoinVertical(lipgloss.Left,
		topSection,
		outputBox,
		inputLine,
	)
}
