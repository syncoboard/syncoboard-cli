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
	ti.Prompt = "" // Remove default textinput prompt as we render a custom one
	ti.Placeholder = "Enter command..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(neonPulse)
	ti.TextStyle = lipgloss.NewStyle().Foreground(syntaxGrey)

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
	// Visual Identity Colors
	obsidianNight = lipgloss.Color("#0B0E14") // Background (optional)
	voidGrey      = lipgloss.Color("#161B22") // Surface
	neonPulse     = lipgloss.Color("#00F5FF") // Accent
	gitGreen      = lipgloss.Color("#2EA043") // Status
	syntaxGrey    = lipgloss.Color("#8B949E") // Typography

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(voidGrey).
			Background(voidGrey).
			Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(neonPulse).
			Underline(true)

	promptStyle = lipgloss.NewStyle().Foreground(neonPulse)
	pathStyle   = lipgloss.NewStyle().Foreground(syntaxGrey)
)

func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	// Calculate panel widths
	panelWidth := (m.width - 6) / 3

	// Max lines for content to fit exactly in Height(10) panel.
	// Lipgloss includes borders in height (2 lines). 1 line for title. Leaving 7 lines.
	maxContentLines := 7

	// Left: Workspaces
	wsContent := titleStyle.Render("Workspaces") + "\n"
	if len(m.workspaces) == 0 {
		wsContent += lipgloss.NewStyle().Foreground(syntaxGrey).Render("No workspaces loaded.\nRun /cd to navigate.")
	} else {
		displayWorkspaces := m.workspaces
		if len(displayWorkspaces) > maxContentLines {
			displayWorkspaces = displayWorkspaces[:maxContentLines]
		}
		wsContent += lipgloss.NewStyle().Foreground(syntaxGrey).Render(strings.Join(displayWorkspaces, "\n"))
	}
	wsPanel := panelStyle.Copy().Width(panelWidth).Height(10).Render(wsContent)

	// Middle: Tasks
	taskContent := titleStyle.Render("Tasks") + "\n"
	if len(m.tasks) == 0 {
		taskContent += lipgloss.NewStyle().Foreground(syntaxGrey).Render("No tasks loaded.")
	} else {
		displayTasks := m.tasks
		if len(displayTasks) > maxContentLines {
			displayTasks = displayTasks[:maxContentLines]
		}
		taskContent += lipgloss.NewStyle().Foreground(syntaxGrey).Render(strings.Join(displayTasks, "\n"))
	}
	taskPanel := panelStyle.Copy().Width(panelWidth).Height(10).Render(taskContent)

	// Right: Task Details
	detailsContent := titleStyle.Render("Task Details") + "\n"
	if m.taskDetails == "" {
		detailsContent += lipgloss.NewStyle().Foreground(syntaxGrey).Render("Select a task to view details.")
	} else {
		lines := strings.Split(m.taskDetails, "\n")
		if len(lines) > maxContentLines {
			lines = lines[:maxContentLines]
		}
		detailsContent += lipgloss.NewStyle().Foreground(syntaxGrey).Render(strings.Join(lines, "\n"))
	}
	detailsPanel := panelStyle.Copy().Width(panelWidth).Height(10).Render(detailsContent)

	topSection := lipgloss.JoinHorizontal(lipgloss.Top, wsPanel, taskPanel, detailsPanel)

	// Output area
	outputBox := panelStyle.Copy().
		Width(m.width - 2).
		Height(m.viewport.Height).
		Render(m.viewport.View())

	// Input area
	prompt := pathStyle.Render(m.virtualPath+" ") + promptStyle.Render("$ ")
	inputLine := lipgloss.NewStyle().Padding(0, 1).Render(prompt + m.textInput.View())

	appStyle := lipgloss.NewStyle().
		Foreground(syntaxGrey).
		Background(obsidianNight)

	viewContent := lipgloss.JoinVertical(lipgloss.Left,
		topSection,
		outputBox,
		inputLine,
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top,
		appStyle.Render(viewContent),
		lipgloss.WithWhitespaceBackground(obsidianNight),
		lipgloss.WithWhitespaceChars(" "),
	)
}
