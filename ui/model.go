package ui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

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
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(ColorNeonPulse).Background(ColorVoidGrey)
	ti.TextStyle = ItemStyle

	vp := viewport.New(0, 0) // Width and Height will be set in WindowSizeMsg
	// We disable up and down to not conflict with command history navigation.
	// We enable page up, page down, half page up (ctrl+up), half page down (ctrl+down).
	vp.KeyMap = viewport.KeyMap{
		PageDown:     key.NewBinding(key.WithKeys("pgdown", " ", "f", "ctrl+f")),
		PageUp:       key.NewBinding(key.WithKeys("pgup", "b", "ctrl+b")),
		HalfPageUp:   key.NewBinding(key.WithKeys("ctrl+up", "u", "ctrl+u")),
		HalfPageDown: key.NewBinding(key.WithKeys("ctrl+down", "d", "ctrl+d")),
		Up:           key.NewBinding(key.WithKeys("")), // disabled
		Down:         key.NewBinding(key.WithKeys("")), // disabled
	}

	return Model{
		virtualPath:    "/",
		textInput:      ti,
		viewport:       vp,
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
	spacer := lipgloss.NewStyle().Background(ColorVoidGrey).Render(strings.Repeat(" ", spacerWidth))
	headerContent := logo + spacer + statusText
	return HeaderStyle.Width(m.width).Render(headerContent)
}

func (m Model) renderWorkspacesColumn(colWidth, topSectionHeight, maxContentLines int) string {
	contentWidth := colWidth - 1 // padding left
	wsTitle := TitleStyle.Width(contentWidth).Render("Workspaces")
	var wsLines []string
	if len(m.workspaces) == 0 {
		wsLines = append(wsLines, ItemStyle.Width(contentWidth).Render("No workspaces loaded."))
		wsLines = append(wsLines, ItemStyle.Width(contentWidth).Render("Run /cd to navigate."))
	} else {
		for i, w := range m.workspaces {
			if i >= maxContentLines {
				break
			}
			wsLines = append(wsLines, ItemStyle.Width(contentWidth).Render(w))
		}
	}
	var allLines []string
	allLines = append(allLines, wsTitle)
	allLines = append(allLines, wsLines...)
	for len(allLines) < topSectionHeight {
		allLines = append(allLines, ItemStyle.Width(contentWidth).Render(strings.Repeat(" ", contentWidth)))
	}
	wsContent := strings.Join(allLines, "\n")
	return lipgloss.NewStyle().Width(colWidth).Height(topSectionHeight).Background(ColorVoidGrey).PaddingLeft(1).Render(wsContent)
}

func (m Model) renderVoiceCallColumn(colWidth, topSectionHeight int) string {
	contentWidth := colWidth*2 - 2 // border left + padding left
	voiceTitle := TitleStyle.Width(contentWidth).Render("Voice Call (Active)")
	voiceLines := []string{
		ItemStyle.Width(contentWidth).Render(fmt.Sprintf("Status: %s", m.voiceState.StatusText)),
		ItemStyle.Width(contentWidth).Render(fmt.Sprintf("Peers: %d", m.voiceState.PeerCount)),
		ItemStyle.Width(contentWidth).Render(fmt.Sprintf("Muted: %v", m.voiceState.IsMuted)),
	}
	if m.voiceState.Error != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Background(ColorVoidGrey).Width(contentWidth)
		voiceLines = append(voiceLines, errStyle.Render(fmt.Sprintf("Error: %s", m.voiceState.Error.Error())))
	}
	var allLines []string
	allLines = append(allLines, voiceTitle)
	allLines = append(allLines, voiceLines...)
	for len(allLines) < topSectionHeight {
		allLines = append(allLines, ItemStyle.Width(contentWidth).Render(strings.Repeat(" ", contentWidth)))
	}
	voiceContent := strings.Join(allLines, "\n")
	return ColumnBorderStyle.Copy().Width(colWidth * 2).Height(topSectionHeight).BorderForeground(ColorNeonPulse).Background(ColorVoidGrey).PaddingLeft(1).Render(voiceContent)
}

func (m Model) renderTasksColumn(colWidth, topSectionHeight, maxContentLines int) string {
	contentWidth := colWidth - 2 // border left + padding left
	taskTitle := TitleStyle.Width(contentWidth).Render("Tasks")
	var taskLines []string
	if len(m.tasks) == 0 {
		taskLines = append(taskLines, ItemStyle.Width(contentWidth).Render("No tasks loaded."))
	} else {
		for i, t := range m.tasks {
			if i >= maxContentLines {
				break
			}
			taskLines = append(taskLines, ItemStyle.Width(contentWidth).Render(t))
		}
	}
	var allLines []string
	allLines = append(allLines, taskTitle)
	allLines = append(allLines, taskLines...)
	for len(allLines) < topSectionHeight {
		allLines = append(allLines, ItemStyle.Width(contentWidth).Render(strings.Repeat(" ", contentWidth)))
	}
	taskContent := strings.Join(allLines, "\n")
	return ColumnBorderStyle.Width(colWidth).Height(topSectionHeight).PaddingLeft(1).Render(taskContent)
}

func (m Model) renderTaskDetailsColumn(colWidth, topSectionHeight, maxContentLines int) string {
	contentWidth := colWidth - 2 // border left + padding left
	detailsTitle := TitleStyle.Width(contentWidth).Render("Task Details")
	var detailLines []string
	if m.taskDetails == "" {
		detailLines = append(detailLines, ItemStyle.Width(contentWidth).Render("Select a task to view details."))
	} else {
		lines := strings.Split(m.taskDetails, "\n")
		for i, l := range lines {
			if i >= maxContentLines {
				break
			}
			clean := ansiRE.ReplaceAllString(l, "")
			if clean == "" {
				clean = " "
			}
			detailLines = append(detailLines, ItemStyle.Width(contentWidth).Render(clean))
		}
	}
	var allLines []string
	allLines = append(allLines, detailsTitle)
	allLines = append(allLines, detailLines...)
	for len(allLines) < topSectionHeight {
		allLines = append(allLines, ItemStyle.Width(contentWidth).Render(strings.Repeat(" ", contentWidth)))
	}
	detailsContent := strings.Join(allLines, "\n")
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
	inputContent := prompt + m.textInput.View()
	contentWidth := lipgloss.Width(inputContent)
	padWidth := m.width - 1 - contentWidth // -1 for left padding
	if padWidth < 0 {
		padWidth = 0
	}
	padStyle := lipgloss.NewStyle().Background(ColorVoidGrey)
	padded := " " + inputContent + padStyle.Render(strings.Repeat(" ", padWidth))
	return lipgloss.NewStyle().Background(ColorVoidGrey).Height(1).Render(padded)
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
