package ui

import (
	"fmt"
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

	// Voice Call State
	voiceEngine *VoiceEngine
	voiceState  VoiceCallState

	err error
}

func InitialModel() Model {
	ti := textinput.New()
	ti.Prompt = "" // Remove default textinput prompt as we render a custom one
	ti.Placeholder = "Enter command..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50
	ti.PlaceholderStyle = ItemStyle.Copy().Foreground(lipgloss.Color("#555555"))
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(ColorNeonPulse)
	ti.TextStyle = ItemStyle

	return Model{
		virtualPath:    "/",
		textInput:      ti,
		outputHistory:  []string{"Welcome to Syncoboard TUI!", "Type /auth to login.", "Type /help to see commands."},
		commandHistory: []string{},
		historyIndex:   -1,
		voiceState:     VoiceCallState{IsActive: false},
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

func (m Model) renderHeader() string {
	logo := LogoStyle.Render("SYNCOBOARD")
	statusText := ItemStyle.Render("Connected")
	if m.activeBoardId != "" {
		statusText = ItemStyle.Render("Board: ") + HighlightStyle.Render(m.activeBoardId)
	}

	headerWidth := m.width - 2 // -2 for padding
	if headerWidth < 0 {
		headerWidth = 0
	}

	// Create space between logo and status
	spacerWidth := headerWidth - lipgloss.Width(logo) - lipgloss.Width(statusText)
	if spacerWidth < 0 {
		spacerWidth = 0
	}
	spacer := strings.Repeat(" ", spacerWidth)
	headerContent := logo + spacer + statusText
	return HeaderStyle.Width(m.width).Render(headerContent)
}

func (m Model) renderWorkspacesColumn(colWidth, topSectionHeight, maxContentLines int) string {
	wsTitle := TitleStyle.Render("Workspaces")
	var wsLines []string
	if len(m.workspaces) == 0 {
		wsLines = append(wsLines, ItemStyle.Render("No workspaces loaded."), ItemStyle.Render("Run /cd to navigate."))
	} else {
		for i, w := range m.workspaces {
			if i >= maxContentLines {
				break
			}
			wsLines = append(wsLines, ItemStyle.Render(w))
		}
	}
	wsContent := lipgloss.JoinVertical(lipgloss.Left, append([]string{wsTitle}, wsLines...)...)
	return lipgloss.NewStyle().Width(colWidth).Height(topSectionHeight).PaddingLeft(1).Render(wsContent)
}

func (m Model) renderVoiceCallColumn(colWidth, topSectionHeight int) string {
	voiceTitle := TitleStyle.Render("Voice Call (Active)")
	voiceLines := []string{
		ItemStyle.Render(fmt.Sprintf("Status: %s", m.voiceState.StatusText)),
		ItemStyle.Render(fmt.Sprintf("Peers: %d", m.voiceState.PeerCount)),
		ItemStyle.Render(fmt.Sprintf("Muted: %v", m.voiceState.IsMuted)),
	}
	if m.voiceState.Error != nil {
		voiceLines = append(voiceLines, lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("Error: %s", m.voiceState.Error.Error())))
	}
	voiceContent := lipgloss.JoinVertical(lipgloss.Left, append([]string{voiceTitle}, voiceLines...)...)
	return ColumnBorderStyle.Copy().Width(colWidth * 2).Height(topSectionHeight).BorderForeground(ColorNeonPulse).PaddingLeft(1).Render(voiceContent)
}

func (m Model) renderTasksColumn(colWidth, topSectionHeight, maxContentLines int) string {
	taskTitle := TitleStyle.Render("Tasks")
	var taskLines []string
	if len(m.tasks) == 0 {
		taskLines = append(taskLines, ItemStyle.Render("No tasks loaded."))
	} else {
		for i, t := range m.tasks {
			if i >= maxContentLines {
				break
			}
			taskLines = append(taskLines, ItemStyle.Render(t))
		}
	}
	taskContent := lipgloss.JoinVertical(lipgloss.Left, append([]string{taskTitle}, taskLines...)...)
	return ColumnBorderStyle.Width(colWidth).Height(topSectionHeight).PaddingLeft(1).Render(taskContent)
}

func (m Model) renderTaskDetailsColumn(colWidth, topSectionHeight, maxContentLines int) string {
	detailsTitle := TitleStyle.Render("Task Details")
	var detailLines []string
	if m.taskDetails == "" {
		detailLines = append(detailLines, ItemStyle.Render("Select a task to view details."))
	} else {
		lines := strings.Split(m.taskDetails, "\n")
		for i, l := range lines {
			if i >= maxContentLines {
				break
			}
			detailLines = append(detailLines, ItemStyle.Render(l))
		}
	}
	detailsContent := lipgloss.JoinVertical(lipgloss.Left, append([]string{detailsTitle}, detailLines...)...)
	return ColumnBorderStyle.Width(colWidth).Height(topSectionHeight).PaddingLeft(1).Render(detailsContent)
}

func (m Model) renderTopSection(colWidth, topSectionHeight, maxContentLines int) string {
	if m.voiceState.IsActive {
		wsCol := m.renderWorkspacesColumn(colWidth, topSectionHeight, maxContentLines)
		voiceCol := m.renderVoiceCallColumn(colWidth, topSectionHeight)
		columns := lipgloss.JoinHorizontal(lipgloss.Top, wsCol, voiceCol)
		return TopSectionStyle.Width(m.width).Height(topSectionHeight).Render(columns)
	}

	wsCol := m.renderWorkspacesColumn(colWidth, topSectionHeight, maxContentLines)
	taskCol := m.renderTasksColumn(colWidth, topSectionHeight, maxContentLines)
	detailsCol := m.renderTaskDetailsColumn(colWidth, topSectionHeight, maxContentLines)
	columns := lipgloss.JoinHorizontal(lipgloss.Top, wsCol, taskCol, detailsCol)
	return TopSectionStyle.Width(m.width).Height(topSectionHeight).Render(columns)
}

func (m Model) renderOutputBox() string {
	return OutputBoxStyle.
		Width(m.width).
		Height(m.viewport.Height).
		Render(m.viewport.View())
}

func (m Model) renderInputLine() string {
	prompt := PathStyle.Render(m.virtualPath+" ") + PromptStyle.Render("$ ")
	return InputLineStyle.Width(m.width).Render(prompt + m.textInput.View())
}

func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	header := m.renderHeader()

	availableWidth := m.width - 2
	if availableWidth < 0 {
		availableWidth = 0
	}
	colWidth := availableWidth / 3

	topSectionHeight := m.height / 3
	if topSectionHeight < 10 {
		topSectionHeight = 10
	}
	maxContentLines := topSectionHeight - 2

	topSection := m.renderTopSection(colWidth, topSectionHeight, maxContentLines)
	outputBox := m.renderOutputBox()
	inputLine := m.renderInputLine()

	viewContent := lipgloss.JoinVertical(lipgloss.Left,
		header,
		topSection,
		outputBox,
		inputLine,
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top,
		AppStyle.Render(viewContent),
		lipgloss.WithWhitespaceBackground(ColorObsidianNight),
		lipgloss.WithWhitespaceChars(" "),
	)
}
