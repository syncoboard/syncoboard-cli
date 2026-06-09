package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/syncoboard/syncoboard/sdks/go/api"
)

var ApiClient *api.Client

func init() {
	baseURL := os.Getenv("API_URL")
	if baseURL == "" {
		baseURL = "https://syncoboard.com/api"
	}
	ApiClient = api.NewClient(baseURL, "")
}

type Config struct {
	Token string `json:"token"`
}

func getConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "syncoboard", "config.json")
}

func LoadConfig() Config {
	var cfg Config
	path := getConfigPath()
	data, err := os.ReadFile(path)
	if err == nil {
		json.Unmarshal(data, &cfg)
	}
	return cfg
}

func SaveConfig(cfg Config) error {
	path := getConfigPath()
	os.MkdirAll(filepath.Dir(path), 0755)
	data, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(path, data, 0644)
}

func openUrl(url string) {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	exec.Command(cmd, args...).Start()
}

type OutputMsg struct {
	Lines []string
}

type AuthSuccessMsg struct {
	Token string
}

func handleAuth() tea.Cmd {
	return func() tea.Msg {
		port := 3456
		authUrl := fmt.Sprintf("https://syncoboard.com/cli/auth?port=%d", port)

		tokenChan := make(chan string)
		errChan := make(chan error)

		server := &http.Server{Addr: fmt.Sprintf(":%d", port)}

		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			token := r.URL.Query().Get("token")
			if token != "" {
				w.Header().Set("Content-Type", "text/html")
				w.Write([]byte("<h1>Authentication successful!</h1><p>You can close this tab and return to the terminal.</p>"))
				go func() {
					tokenChan <- token
				}()
			} else {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Bad Request"))
			}
		})

		go func() {
			err := server.ListenAndServe()
			if err != nil && err != http.ErrServerClosed {
				errChan <- err
			}
		}()

		openUrl(authUrl)

		select {
		case token := <-tokenChan:
			server.Close()
			return AuthSuccessMsg{Token: token}
		case <-errChan:
			// Just open the URL if server fails (e.g. port in use)
			openUrl(authUrl)
			return OutputMsg{Lines: []string{"Server error or port in use. Opening browser..."}}
		}
	}
}

func resolvePath(basePath, target string) string {
	if target == "" || target == "." {
		return basePath
	}
	if target == "~" || target == "/" {
		return "/"
	}
	if strings.HasPrefix(target, "/") {
		return target
	}

	baseParts := strings.Split(strings.Trim(basePath, "/"), "/")
	if len(baseParts) == 1 && baseParts[0] == "" {
		baseParts = []string{}
	}

	targetParts := strings.Split(target, "/")

	for _, part := range targetParts {
		if part == ".." {
			if len(baseParts) > 0 {
				baseParts = baseParts[:len(baseParts)-1]
			}
		} else if part != "." && part != "" {
			baseParts = append(baseParts, part)
		}
	}

	return "/" + strings.Join(baseParts, "/")
}

func executeCommand(m Model, input string) (Model, tea.Cmd) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return m, nil
	}

	cmdName := parts[0]
	if strings.HasPrefix(cmdName, "/") {
		cmdName = cmdName[1:]
	}
	args := parts[1:]

	var cmd tea.Cmd

	switch cmdName {
	case "auth":
		m.outputHistory = append(m.outputHistory, "Waiting for authentication callback on port 3456...")
		cmd = handleAuth()
	case "logout":
		cfg := LoadConfig()
		cfg.Token = ""
		SaveConfig(cfg)
		ApiClient.Token = ""
		m.outputHistory = append(m.outputHistory, "Logged out.")
	case "clear":
		m.outputHistory = []string{}
	case "pwd":
		m.outputHistory = append(m.outputHistory, m.virtualPath)
	case "ls":
		cmd = handleLs(m.virtualPath, args)
	case "cd":
		cmd = handleCd(m.virtualPath, args)
	case "help":
		cmd = handleHelp()
	case "delete-workspace":
		cmd = handleDeleteWorkspace(args)
	case "activate-workspace":
		cmd = handleUpdateWorkspaceStatus(args, true)
	case "deactivate-workspace":
		cmd = handleUpdateWorkspaceStatus(args, false)
	case "restore-workspace":
		cmd = handleRestoreWorkspace(args)
	case "list-deleted-boards":
		cmd = handleListDeletedBoards()
	case "delete-board":
		cmd = handleDeleteBoard(args)
	case "restore-board":
		cmd = handleRestoreBoard(args)
	case "activate-board":
		cmd = handleUpdateBoardStatus(args, true)
	case "deactivate-board":
		cmd = handleUpdateBoardStatus(args, false)
	case "invite-member":
		cmd = handleInviteMember(args)
	case "rmv-member":
		cmd = handleRemoveMember(args)
	case "list-tasks":
		cmd = handleListTasks(m.virtualPath, args)
	case "add-task":
		cmd = handleAddTask(m.activeBoardId, args)
	case "update-task":
		cmd = handleUpdateTask(args)
	case "delete-task":
		cmd = handleDeleteTask(args)
	case "select-task":
		if len(args) > 0 {
			m.selectedTaskId = args[0]
			m.outputHistory = append(m.outputHistory, fmt.Sprintf("Selected task SYNC-%s", args[0]))
			cmd = fetchTaskDetails(args[0])
		} else {
			m.outputHistory = append(m.outputHistory, "Error: Missing task id. Usage: /select-task <taskId>")
		}
	case "search-task":
		m.outputHistory = append(m.outputHistory, "Searching tasks... (Not fully implemented in TUI without visual representation)")
	case "report-bug":
		cmd = handleReportBug(args)
	case "updates":
		cmd = handleUpdates()
	case "join-voice-call":
		cmd = handleJoinVoiceCall(m)
	case "tui", "classic", "board", "dashboard", "back", "forward", "settings", "add-board":
		m.outputHistory = append(m.outputHistory, "Not supported in this version.")
	default:
		m.outputHistory = append(m.outputHistory, fmt.Sprintf("Command not found: %s", cmdName))
	}

	return m, cmd
}

// Command Handlers

func handleLs(virtualPath string, args []string) tea.Cmd {
	return func() tea.Msg {
		targetPath := "."
		if len(args) > 0 {
			targetPath = args[0]
		}
		resolvedPath := resolvePath(virtualPath, targetPath)

		resp, err := ApiClient.Directory.GetDirectory(resolvedPath)
		if err != nil {
			return OutputMsg{Lines: []string{"Error: " + err.Error()}}
		}

		if len(resp.Entries) == 0 {
			return OutputMsg{Lines: []string{}}
		}

		var lines []string
		if resp.Type == "Board" {
			groupedTasks := make(map[string][]api.DirectoryEntry)
			for _, entry := range resp.Entries {
				if entry.Type == "Task" && entry.Status != "" {
					groupedTasks[entry.Status] = append(groupedTasks[entry.Status], entry)
				}
			}

			// Predefined order for statuses
			statuses := []string{"TODO", "IN_PROGRESS", "IN_REVIEW", "CHANGES_REQUESTED", "DONE", "CLOSED"}
			for _, status := range statuses {
				if tasks, ok := groupedTasks[status]; ok {
					lines = append(lines, "--- "+status+" ---")
					for _, entry := range tasks {
						title := entry.Title
						if len(title) > 30 {
							title = title[:30] + "..."
						}
						lines = append(lines, fmt.Sprintf("%s (%s) [%s]", entry.Name, title, entry.Type))
					}
					if resp.HasMoreByStatus[status] {
						lines = append(lines, "...")
					}
				}
			}
		} else {
			for _, entry := range resp.Entries {
				if entry.Type == "Task" {
					title := entry.Title
					if len(title) > 30 {
						title = title[:30] + "..."
					}
					lines = append(lines, fmt.Sprintf("%s (%s) [%s]", entry.Name, title, entry.Type))
				} else {
					formattedName := strings.ReplaceAll(strings.ToLower(entry.Name), " ", "-")
					lines = append(lines, fmt.Sprintf("%s [%s]", formattedName, entry.Type))
				}
			}
		}
		return OutputMsg{Lines: lines}
	}
}

type CdResultMsg struct {
	Path string
	Type string
	ID   string
}

func handleCd(virtualPath string, args []string) tea.Cmd {
	return func() tea.Msg {
		targetPath := "~"
		if len(args) > 0 {
			targetPath = args[0]
		}
		resolvedPath := resolvePath(virtualPath, targetPath)

		resp, err := ApiClient.Directory.GetDirectory(resolvedPath)
		if err != nil {
			return OutputMsg{Lines: []string{"Error: " + err.Error()}}
		}

		return CdResultMsg{
			Path: resolvedPath,
			Type: resp.Type,
			ID:   resp.ID,
		}
	}
}

func handleJoinVoiceCall(m Model) tea.Cmd {
	return func() tea.Msg {
		if m.activeBoardId == "" {
			return OutputMsg{Lines: []string{"Error: You must be in a board directory to join a voice call. Use /cd to navigate to a board."}}
		}
		return StartVoiceCallMsg{BoardID: m.activeBoardId}
	}
}

func handleHelp() tea.Cmd {
	return func() tea.Msg {
		lines := []string{
			"--- SYNC-OS TUI v1.0.0 ---",
			"Navigation & System:",
			"  /ls          - List directory contents",
			"  /cd          - Change directory",
			"  /pwd         - Print working directory",
			"  /help        - List commands",
			"  /auth        - Login",
			"  /logout      - Logout",
			"  /clear       - Clear output",
			"",
			"Workspaces:",
			"  /delete-workspace",
			"  /activate-workspace",
			"  /deactivate-workspace",
			"  /restore-workspace",
			"",
			"Boards:",
			"  /delete-board",
			"  /restore-board",
			"  /list-deleted-boards",
			"  /activate-board",
			"  /deactivate-board",
			"  /invite-member",
			"  /rmv-member",
			"",
			"Tasks:",
			"  /list-tasks",
			"  /add-task",
			"  /update-task",
			"  /delete-task",
			"  /select-task",
			"",
			"Other:",
			"  /updates",
			"  /report-bug",
		}
		return OutputMsg{Lines: lines}
	}
}

func handleDeleteWorkspace(args []string) tea.Cmd {
	return func() tea.Msg {
		if len(args) == 0 {
			return OutputMsg{Lines: []string{"Error: Missing arguments. Usage: /delete-workspace <workspace_name>"}}
		}
		name := strings.Join(args, " ")
		_, err := ApiClient.Workspace.DeleteWorkspace(name)
		if err != nil {
			return OutputMsg{Lines: []string{"Error: " + err.Error()}}
		}
		return OutputMsg{Lines: []string{"Successfully deleted workspace '" + name + "'."}}
	}
}

func handleUpdateWorkspaceStatus(args []string, isActive bool) tea.Cmd {
	return func() tea.Msg {
		action := "activate"
		if !isActive {
			action = "deactivate"
		}
		if len(args) == 0 {
			return OutputMsg{Lines: []string{fmt.Sprintf("Error: Missing arguments. Usage: /%s-workspace <workspace_name>", action)}}
		}
		name := strings.Join(args, " ")
		_, err := ApiClient.Workspace.UpdateWorkspaceStatus(name, isActive)
		if err != nil {
			return OutputMsg{Lines: []string{"Error: " + err.Error()}}
		}
		return OutputMsg{Lines: []string{fmt.Sprintf("Successfully %sd workspace '%s'.", action, name)}}
	}
}

func handleRestoreWorkspace(args []string) tea.Cmd {
	return func() tea.Msg {
		if len(args) == 0 {
			return OutputMsg{Lines: []string{"Error: Missing arguments. Usage: /restore-workspace <workspace_name>"}}
		}
		name := args[0]
		_, err := ApiClient.Workspace.RestoreWorkspace(name)
		if err != nil {
			return OutputMsg{Lines: []string{"Error: " + err.Error()}}
		}
		return OutputMsg{Lines: []string{"Successfully restored workspace '" + name + "'."}}
	}
}

func handleListDeletedBoards() tea.Cmd {
	return func() tea.Msg {
		boards, err := ApiClient.Board.GetDeletedBoards()
		if err != nil {
			return OutputMsg{Lines: []string{"Error: " + err.Error()}}
		}
		if len(boards) == 0 {
			return OutputMsg{Lines: []string{"No soft-deleted boards found."}}
		}
		lines := []string{"--- Deleted Boards ---"}
		for _, b := range boards {
			lines = append(lines, fmt.Sprintf("- %s/%s (Repo: %s) | %s until permanent deletion", b.WorkspaceName, b.Name, b.RepositoryName, b.TimeLeftString))
		}
		return OutputMsg{Lines: lines}
	}
}

func parseBoardArgs(args []string, cmd string) (string, string, error) {
	if len(args) == 0 {
		return "", "", fmt.Errorf("Missing arguments. Usage: /%s <workspace_name>/<board_name>", cmd)
	}
	fullPath := strings.Join(args, " ")
	parts := strings.Split(fullPath, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("Invalid format. Usage: /%s <workspace_name>/<board_name>", cmd)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

func handleDeleteBoard(args []string) tea.Cmd {
	return func() tea.Msg {
		ws, board, err := parseBoardArgs(args, "delete-board")
		if err != nil {
			return OutputMsg{Lines: []string{"Error: " + err.Error()}}
		}
		_, err = ApiClient.Board.DeleteBoard(ws, board)
		if err != nil {
			return OutputMsg{Lines: []string{"Error: " + err.Error()}}
		}
		return OutputMsg{Lines: []string{fmt.Sprintf("Successfully deleted board '%s' from workspace '%s'.", board, ws)}}
	}
}

func handleRestoreBoard(args []string) tea.Cmd {
	return func() tea.Msg {
		ws, board, err := parseBoardArgs(args, "restore-board")
		if err != nil {
			return OutputMsg{Lines: []string{"Error: " + err.Error()}}
		}
		_, err = ApiClient.Board.RestoreBoard(ws, board)
		if err != nil {
			return OutputMsg{Lines: []string{"Error: " + err.Error()}}
		}
		return OutputMsg{Lines: []string{fmt.Sprintf("Successfully restored board '%s' in workspace '%s'.", board, ws)}}
	}
}

func handleUpdateBoardStatus(args []string, isActive bool) tea.Cmd {
	return func() tea.Msg {
		action := "activate"
		if !isActive {
			action = "deactivate"
		}
		ws, board, err := parseBoardArgs(args, action+"-board")
		if err != nil {
			return OutputMsg{Lines: []string{"Error: " + err.Error()}}
		}
		_, err = ApiClient.Board.UpdateBoardStatus(ws, board, isActive)
		if err != nil {
			return OutputMsg{Lines: []string{"Error: " + err.Error()}}
		}
		return OutputMsg{Lines: []string{fmt.Sprintf("Successfully %sd board '%s' in workspace '%s'.", action, board, ws)}}
	}
}

func handleInviteMember(args []string) tea.Cmd {
	return func() tea.Msg {
		if len(args) < 2 {
			return OutputMsg{Lines: []string{"Error: Missing arguments. Usage: /invite-member <workspace_name>/<board_name> <user_id_or_email>"}}
		}
		parts := strings.Split(args[0], "/")
		if len(parts) != 2 {
			return OutputMsg{Lines: []string{"Error: Invalid format. Usage: /invite-member <workspace_name>/<board_name> <user_id_or_email>"}}
		}
		ws, board, identifier := parts[0], parts[1], args[1]
		_, err := ApiClient.Board.InviteMember(ws, board, identifier)
		if err != nil {
			return OutputMsg{Lines: []string{"Error: " + err.Error()}}
		}
		return OutputMsg{Lines: []string{fmt.Sprintf("Successfully invited member '%s' to board '%s'.", identifier, board)}}
	}
}

func handleRemoveMember(args []string) tea.Cmd {
	return func() tea.Msg {
		if len(args) < 2 {
			return OutputMsg{Lines: []string{"Error: Missing arguments. Usage: /rmv-member <workspace_name>/<board_name> <user_id_or_email>"}}
		}
		parts := strings.Split(args[0], "/")
		if len(parts) != 2 {
			return OutputMsg{Lines: []string{"Error: Invalid format. Usage: /rmv-member <workspace_name>/<board_name> <user_id_or_email>"}}
		}
		ws, board, identifier := parts[0], parts[1], args[1]
		_, err := ApiClient.Board.RemoveMember(ws, board, identifier)
		if err != nil {
			return OutputMsg{Lines: []string{"Error: " + err.Error()}}
		}
		return OutputMsg{Lines: []string{fmt.Sprintf("Successfully removed member '%s' from board '%s'.", identifier, board)}}
	}
}

func handleListTasks(virtualPath string, args []string) tea.Cmd {
	return func() tea.Msg {
		if len(args) == 0 {
			return OutputMsg{Lines: []string{"Error: Missing arguments. Usage: /list-tasks <workspace>/<board> [page] [limit]"}}
		}

		resolvedPath := resolvePath(virtualPath, args[0])
		normalizedPath := strings.TrimRight(strings.ReplaceAll(resolvedPath, "//", "/"), "/")
		if !strings.HasPrefix(normalizedPath, "/") {
			normalizedPath = "/" + normalizedPath
		}
		parts := strings.Split(strings.Trim(normalizedPath, "/"), "/")
		if len(parts) != 2 {
			return OutputMsg{Lines: []string{"Error: Path must point to a specific board, e.g., <workspace>/<board>"}}
		}
		ws, board := parts[0], parts[1]

		page, limit := 1, 5
		if len(args) > 1 {
			fmt.Sscanf(args[1], "%d", &page)
		}
		if len(args) > 2 {
			fmt.Sscanf(args[2], "%d", &limit)
		}

		resp, err := ApiClient.Task.ListTasks(ws, board, page, limit)
		if err != nil {
			return OutputMsg{Lines: []string{"Error: " + err.Error()}}
		}

		var lines []string
		lines = append(lines, fmt.Sprintf("Listing tasks for board: %s/%s (Page %d, Limit %d)", ws, board, page, limit))
		total := 0
		statuses := []string{"TODO", "IN_PROGRESS", "IN_REVIEW", "CHANGES_REQUESTED", "DONE", "CLOSED"}

		for _, status := range statuses {
			if tasks, ok := resp.TasksByStatus[status]; ok && len(tasks) > 0 {
				total += len(tasks)
				lines = append(lines, "--- "+status+" ---")
				for _, task := range tasks {
					title := task.Title
					if len(title) > 30 {
						title = title[:30] + "..."
					}
					lines = append(lines, fmt.Sprintf("SYNC-%s (%s) [Task]", task.ID, title))
				}
				if resp.HasMoreByStatus[status] {
					lines = append(lines, "... (more tasks available on next page)")
				}
			}
		}
		if total == 0 {
			lines = append(lines, "No tasks found on this board for the given page.")
		}
		return OutputMsg{Lines: lines}
	}
}

func handleAddTask(activeBoardId string, args []string) tea.Cmd {
	return func() tea.Msg {
		if activeBoardId == "" {
			return OutputMsg{Lines: []string{"Error: You must be on a board to add a task."}}
		}
		if len(args) == 0 {
			return OutputMsg{Lines: []string{"Error: Missing task title. Usage: /add-task <title>"}}
		}
		title := strings.Join(args, " ")
		_, err := ApiClient.Task.AddTask(api.CreateTaskPayload{BoardID: activeBoardId, Title: title})
		if err != nil {
			return OutputMsg{Lines: []string{"Error: " + err.Error()}}
		}
		return OutputMsg{Lines: []string{"Successfully added task: '" + title + "'"}}
	}
}

func handleUpdateTask(args []string) tea.Cmd {
	return func() tea.Msg {
		if len(args) < 2 {
			return OutputMsg{Lines: []string{"Error: Missing arguments. Usage: /update-task <task_id> <status>"}}
		}
		taskId := args[0]
		statusRaw := strings.Join(args[1:], " ")
		status := strings.ToUpper(strings.ReplaceAll(statusRaw, "-", "_"))
		status = strings.ReplaceAll(status, " ", "_")

		valid := false
		statuses := []string{"TODO", "IN_PROGRESS", "IN_REVIEW", "CHANGES_REQUESTED", "DONE", "CLOSED"}
		for _, s := range statuses {
			if s == status {
				valid = true
				break
			}
		}
		if !valid {
			return OutputMsg{Lines: []string{fmt.Sprintf("Error: Invalid status '%s'.", statusRaw)}}
		}

		_, err := ApiClient.Task.UpdateTaskStatus(taskId, status)
		if err != nil {
			return OutputMsg{Lines: []string{"Error: " + err.Error()}}
		}
		return OutputMsg{Lines: []string{fmt.Sprintf("Successfully updated task SYNC-%s to %s", taskId, status)}}
	}
}

func handleDeleteTask(args []string) tea.Cmd {
	return func() tea.Msg {
		if len(args) == 0 {
			return OutputMsg{Lines: []string{"Error: Missing task id. Usage: /delete-task <task_id>"}}
		}
		taskId := args[0]
		_, err := ApiClient.Task.DeleteTask(taskId)
		if err != nil {
			return OutputMsg{Lines: []string{"Error: " + err.Error()}}
		}
		return OutputMsg{Lines: []string{fmt.Sprintf("Successfully deleted task SYNC-%s", taskId)}}
	}
}

func handleReportBug(args []string) tea.Cmd {
	return func() tea.Msg {
		if len(args) == 0 {
			return OutputMsg{Lines: []string{"Error: Missing message. Usage: /report-bug <message>"}}
		}
		msg := strings.Join(args, " ")
		_, err := ApiClient.Bug.ReportBug(api.BugReportPayload{Message: msg})
		if err != nil {
			return OutputMsg{Lines: []string{"Error: " + err.Error()}}
		}
		return OutputMsg{Lines: []string{"Bug reported successfully. Thank you!"}}
	}
}

func handleUpdates() tea.Cmd {
	return func() tea.Msg {
		logsData, err := ApiClient.Notification.GetNotifications()
		if err != nil {
			return OutputMsg{Lines: []string{"Error: " + err.Error()}}
		}
		readData, err := ApiClient.Notification.GetReadState()
		if err != nil {
			return OutputMsg{Lines: []string{"Error: " + err.Error()}}
		}

		var readDate time.Time
		hasReadDate := false
		if readData.LastRead != nil && *readData.LastRead != "" {
			parsedDate, err := time.Parse(time.RFC3339, *readData.LastRead)
			if err == nil {
				readDate = parsedDate
				hasReadDate = true
			}
		}

		var lines []string
		for _, log := range logsData {
			if hasReadDate && !log.CreatedAt.After(readDate) {
				continue
			}

			dateStr := log.CreatedAt.Format("2006-01-02")
			switch log.Type {
			case "INVITATION":
				name := "Someone"
				if log.Actor != nil {
					name = log.Actor.Name
					if name == "" {
						name = log.Actor.Email
					}
				}
				boardName := "Unknown"
				if log.Board != nil && log.Board.Workspace != nil {
					boardName = log.Board.Workspace.Name + "/" + log.Board.Name
				}
				lines = append(lines, fmt.Sprintf("[%s] %s invited you to join %s", dateStr, name, boardName))
			case "MEMBER_JOIN":
				name := "A new member"
				if log.Actor != nil {
					name = log.Actor.Name
				}
				boardName := "Unknown"
				if log.Board != nil {
					boardName = log.Board.Workspace.Name + "/" + log.Board.Name
				}
				lines = append(lines, fmt.Sprintf("[%s] %s joined %s", dateStr, name, boardName))
			case "MEMBER_LEAVE":
				name := "A member"
				if log.Actor != nil {
					name = log.Actor.Name
				}
				boardName := "Unknown"
				if log.Board != nil {
					boardName = log.Board.Workspace.Name + "/" + log.Board.Name
				}
				lines = append(lines, fmt.Sprintf("[%s] %s left %s", dateStr, name, boardName))
			case "TASK_UPDATE":
				taskTitle := "Unknown Task"
				status := ""
				if log.Task != nil {
					taskTitle = log.Task.Title
					status = log.Task.Status
				}
				boardName := "Unknown"
				if log.Board != nil {
					boardName = log.Board.Workspace.Name + "/" + log.Board.Name
				}
				lines = append(lines, fmt.Sprintf("[%s] Task \"%s\" was updated to %s in %s", dateStr, taskTitle, status, boardName))
			}
		}

		if len(lines) == 0 {
			return OutputMsg{Lines: []string{"No new updates"}}
		}

		ApiClient.Notification.MarkAsRead()
		return OutputMsg{Lines: lines}
	}
}

// Command tab completion
func handleTabCompletion(m Model) (Model, tea.Cmd) {
	input := m.textInput.Value()
	if input == "" {
		return m, nil
	}

	parts := strings.Split(input, " ")

	registryKeys := []string{
		"ls", "cd", "pwd", "help", "logout", "clear", "tui", "classic",
		"delete-workspace", "activate-workspace", "deactivate-workspace",
		"restore-workspace", "delete-board", "restore-board", "list-deleted-boards",
		"activate-board", "deactivate-board", "invite-member", "rmv-member",
		"list-tasks", "add-task", "update-task", "delete-task", "search-task", "select-task",
		"report-bug", "updates", "auth", "join-voice-call",
	}

	if len(parts) == 1 {
		// Command completion
		cmdPart := parts[0]
		prefix := ""
		if strings.HasPrefix(cmdPart, "/") {
			prefix = "/"
			cmdPart = cmdPart[1:]
		}

		var matches []string
		for _, k := range registryKeys {
			if strings.HasPrefix(k, cmdPart) {
				matches = append(matches, k)
			}
		}

		if len(matches) == 1 {
			m.textInput.SetValue(prefix + matches[0] + " ")
			m.textInput.SetCursor(len(m.textInput.Value()))
		} else if len(matches) > 1 {
			m.outputHistory = append(m.outputHistory, strings.Join(matches, "  "))
		}
		return m, nil
	}

	// For ls and cd, do path completion
	cmdName := parts[0]
	if strings.HasPrefix(cmdName, "/") {
		cmdName = cmdName[1:]
	}

	if cmdName == "cd" || cmdName == "ls" {
		targetPath := parts[len(parts)-1]

		return m, func() tea.Msg {
			dirPath := targetPath
			prefix := ""

			lastSlash := strings.LastIndex(targetPath, "/")
			if lastSlash >= 0 {
				dirPath = targetPath[:lastSlash]
				if dirPath == "" {
					dirPath = "/"
				}
				prefix = targetPath[lastSlash+1:]
			} else {
				dirPath = "."
				prefix = targetPath
			}

			resolvedPath := resolvePath(m.virtualPath, dirPath)

			resp, err := ApiClient.Directory.GetDirectory(resolvedPath)
			if err != nil {
				return nil
			}

			var matches []string
			for _, entry := range resp.Entries {
				formattedName := entry.Name
				if entry.Type != "Task" && entry.Type != "Board" {
					formattedName = strings.ReplaceAll(strings.ToLower(entry.Name), " ", "-")
				}

				if strings.HasPrefix(strings.ToLower(formattedName), strings.ToLower(prefix)) {
					matches = append(matches, formattedName)
				}
			}

			if len(matches) == 1 {
				newPath := ""
				if lastSlash >= 0 {
					newPath = targetPath[:lastSlash+1] + matches[0]
				} else {
					newPath = matches[0]
				}

				// Reconstruct input
				parts[len(parts)-1] = newPath
				return TabCompletionMsg{
					Input: strings.Join(parts, " "),
				}
			} else if len(matches) > 1 {
				return OutputMsg{Lines: []string{strings.Join(matches, "  ")}}
			}

			return nil
		}
	}

	return m, nil
}

type TabCompletionMsg struct {
	Input string
}

// Background UI Update Commands

type WorkspacesMsg struct {
	Names []string
	Err   error
}

type UpdateLastOnlineMsg struct {
	Err error
}

func updateLastOnlineCmd() tea.Cmd {
	return func() tea.Msg {
		err := ApiClient.User.UpdateLastOnline()
		return UpdateLastOnlineMsg{Err: err}
	}
}

func fetchWorkspaces() tea.Cmd {
	return func() tea.Msg {
		wsData, err := ApiClient.Workspace.GetUserWorkspaces(false)
		if err != nil {
			return WorkspacesMsg{Err: err}
		}

		var names []string

		// Type assert since it returns interface{}
		if wsList, ok := wsData.([]interface{}); ok {
			for _, wItem := range wsList {
				if wMap, ok := wItem.(map[string]interface{}); ok {
					if name, ok := wMap["name"].(string); ok {
						names = append(names, name)
					}
				}
			}
		}

		return WorkspacesMsg{Names: names}
	}
}

type TasksMsg struct {
	Tasks []string
	Err   error
}

func fetchTasks(virtualPath string) tea.Cmd {
	return func() tea.Msg {
		normalizedPath := strings.TrimRight(strings.ReplaceAll(virtualPath, "//", "/"), "/")
		if !strings.HasPrefix(normalizedPath, "/") {
			normalizedPath = "/" + normalizedPath
		}
		parts := strings.Split(strings.Trim(normalizedPath, "/"), "/")

		if len(parts) >= 2 {
			ws, board := parts[0], parts[1]
			resp, err := ApiClient.Task.ListTasks(ws, board, 1, 50)
			if err != nil {
				return TasksMsg{Err: err}
			}
			var lines []string
			for _, statusTasks := range resp.TasksByStatus {
				for _, task := range statusTasks {
					title := task.Title
					if len(title) > 30 {
						title = title[:30] + "..."
					}
					lines = append(lines, fmt.Sprintf("SYNC-%s: %s", task.ID, title))
				}
			}
			return TasksMsg{Tasks: lines}
		}
		return TasksMsg{Tasks: []string{}}
	}
}

type TaskDetailsMsg struct {
	Details string
	Err     error
}

func fetchTaskDetails(taskId string) tea.Cmd {
	return func() tea.Msg {
		task, err := ApiClient.Task.GetTask(taskId)
		if err != nil {
			return TaskDetailsMsg{Err: err}
		}

		var assignees []string
		for _, a := range task.Assignees {
			if a.Name != "" {
				assignees = append(assignees, a.Name)
			} else {
				assignees = append(assignees, a.Email)
			}
		}
		assigneeStr := "None"
		if len(assignees) > 0 {
			assigneeStr = strings.Join(assignees, ", ")
		}

		details := lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("SYNC-%s", task.ID)) + "\n"
		details += fmt.Sprintf("Title: %s\n", task.Title)
		details += fmt.Sprintf("Status: %s\n", task.Status)
		if task.Description != "" {
			details += fmt.Sprintf("Desc: %s\n", task.Description)
		}
		details += lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render(fmt.Sprintf("\nAssignees: %s", assigneeStr))

		return TaskDetailsMsg{Details: details}
	}
}

type StartVoiceCallMsg struct {
	BoardID string
}

type VoiceCallUpdateMsg struct {
	State VoiceCallState
}

func listenToVoiceEngine(engine *VoiceEngine) tea.Cmd {
	return func() tea.Msg {
		state := <-engine.StateChan
		return VoiceCallUpdateMsg{State: state}
	}
}

// Update the Model's Update method to handle these messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			cmdStr := m.textInput.Value()
			if strings.TrimSpace(cmdStr) != "" {
				m.commandHistory = append([]string{cmdStr}, m.commandHistory...)
				if len(m.commandHistory) > 100 {
					m.commandHistory = m.commandHistory[:100]
				}
				m.outputHistory = append(m.outputHistory, "> "+cmdStr)

				if cmdStr == "/leave-voice-call" && m.voiceEngine != nil {
					m.voiceEngine.Stop()
					m.voiceEngine = nil
					m.voiceState.IsActive = false
					m.outputHistory = append(m.outputHistory, "Left voice call.")
				} else if cmdStr == "/mute" && m.voiceEngine != nil {
					m.voiceEngine.ToggleMute()
				} else {
					m, cmd = executeCommand(m, cmdStr)
					cmds = append(cmds, cmd)
				}
			}
			m.textInput.SetValue("")
			m.historyIndex = -1

			m.viewport.SetContent(strings.Join(m.outputHistory, "\n"))
			m.viewport.GotoBottom()

		case tea.KeyUp:
			if len(m.commandHistory) > 0 {
				m.historyIndex++
				if m.historyIndex >= len(m.commandHistory) {
					m.historyIndex = len(m.commandHistory) - 1
				}
				m.textInput.SetValue(m.commandHistory[m.historyIndex])
				m.textInput.SetCursor(len(m.textInput.Value()))
			}
		case tea.KeyDown:
			if m.historyIndex > -1 {
				m.historyIndex--
				if m.historyIndex == -1 {
					m.textInput.SetValue("")
				} else {
					m.textInput.SetValue(m.commandHistory[m.historyIndex])
					m.textInput.SetCursor(len(m.textInput.Value()))
				}
			}
		case tea.KeyTab:
			m, cmd = handleTabCompletion(m)
			cmds = append(cmds, cmd)
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 15
		m.viewport.SetContent(strings.Join(m.outputHistory, "\n"))
		m.viewport.GotoBottom()

	case OutputMsg:
		m.outputHistory = append(m.outputHistory, msg.Lines...)
		m.viewport.SetContent(strings.Join(m.outputHistory, "\n"))
		m.viewport.GotoBottom()

	case AuthSuccessMsg:
		m.outputHistory = append(m.outputHistory, "Authentication successful!")
		cfg := LoadConfig()
		cfg.Token = msg.Token
		SaveConfig(cfg)
		ApiClient.Token = msg.Token
		m.viewport.SetContent(strings.Join(m.outputHistory, "\n"))
		m.viewport.GotoBottom()

	case CdResultMsg:
		m.virtualPath = msg.Path
		m.outputHistory = append(m.outputHistory, fmt.Sprintf("Changed directory to %s", msg.Path))
		if msg.Type == "Board" {
			m.activeBoardId = msg.ID
			m.selectedTaskId = ""
			m.taskDetails = ""
			cmds = append(cmds, fetchTasks(m.virtualPath))
		} else if msg.Type == "Task" {
			m.selectedTaskId = msg.ID
			cmds = append(cmds, fetchTaskDetails(m.selectedTaskId))
		} else {
			m.activeBoardId = ""
			m.selectedTaskId = ""
			m.taskDetails = ""
			m.tasks = []string{}
		}
		m.viewport.SetContent(strings.Join(m.outputHistory, "\n"))
		m.viewport.GotoBottom()

	case WorkspacesMsg:
		if msg.Err == nil {
			m.workspaces = msg.Names
		} else {
			m.outputHistory = append(m.outputHistory, fmt.Sprintf("Error fetching workspaces: %v", msg.Err))
			m.viewport.SetContent(strings.Join(m.outputHistory, "\n"))
			m.viewport.GotoBottom()
		}

	case TasksMsg:
		if msg.Err == nil {
			m.tasks = msg.Tasks
		} else {
			m.outputHistory = append(m.outputHistory, fmt.Sprintf("Error fetching tasks: %v", msg.Err))
			m.viewport.SetContent(strings.Join(m.outputHistory, "\n"))
			m.viewport.GotoBottom()
		}

	case TaskDetailsMsg:
		if msg.Err == nil {
			m.taskDetails = msg.Details
		} else {
			m.outputHistory = append(m.outputHistory, fmt.Sprintf("Error fetching task details: %v", msg.Err))
			m.viewport.SetContent(strings.Join(m.outputHistory, "\n"))
			m.viewport.GotoBottom()
		}

	case StartVoiceCallMsg:
		if m.voiceEngine != nil {
			m.voiceEngine.Stop()
		}
		vcClient := NewVoiceClient(ApiClient)
		m.voiceEngine = NewVoiceEngine(msg.BoardID, vcClient)
		err := m.voiceEngine.Start()
		if err != nil {
			m.outputHistory = append(m.outputHistory, fmt.Sprintf("Error starting voice call: %v", err))
			m.voiceEngine = nil
			m.viewport.SetContent(strings.Join(m.outputHistory, "\n"))
			m.viewport.GotoBottom()
		} else {
			m.outputHistory = append(m.outputHistory, "Joining voice call... Type /leave-voice-call to exit, or /mute to toggle microphone.")
			m.viewport.SetContent(strings.Join(m.outputHistory, "\n"))
			m.viewport.GotoBottom()
			cmds = append(cmds, listenToVoiceEngine(m.voiceEngine))
		}

	case VoiceCallUpdateMsg:
		m.voiceState = msg.State
		if m.voiceState.IsActive && m.voiceEngine != nil {
			cmds = append(cmds, listenToVoiceEngine(m.voiceEngine))
		}

	case TabCompletionMsg:
		m.textInput.SetValue(msg.Input)
		m.textInput.SetCursor(len(m.textInput.Value()))
	}

	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}
