package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/biodoia/framegotui/pkg/core"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/biodoia/golem/internal/config"
	"github.com/biodoia/golem/internal/session"
	"github.com/biodoia/golem/internal/tools"
	"github.com/biodoia/golem/pkg/zhipu"
)

type Model struct {
	input          string
	width          int
	height         int
	loading        bool
	model          string
	apiKey         string
	client         *zhipu.Client
	cmds           map[string]*tools.Command
	ready          bool
	theme          lipgloss.Style
	streamText     <-chan string
	streamErr      <-chan error
	sessions       *session.SessionManager
	currentSession *session.Session
	statusMessage  string
}

type Message struct {
	Role    string
	Content string
	Time    time.Time
}

type responseMsg struct{ text string }

type errorMsg struct{ err error }

type streamingMsg struct{ text string }

type startStreamMsg struct {
	textCh <-chan string
	errCh  <-chan error
}

type streamDoneMsg struct{}

type sessionMsg struct {
	session *session.Session
}

type statusMsg struct{ text string }

func NewAppModel(settings config.Settings) Model {
	client := zhipu.NewClient(settings.APIKey)
	cmds := tools.Commands()
	extCmds := tools.LoadExternalCommands(config.CommandsSearchPaths(settings.CommandsPath))
	for k, v := range extCmds {
		cmds[k] = v
	}

	sm, _ := session.NewSessionManager()
	currentSession := sm.Current()
	if currentSession == nil {
		currentSession = sm.CreateSession("New Chat", settings.Model)
	}

	return Model{
		model:          settings.Model,
		apiKey:         settings.APIKey,
		client:         client,
		cmds:           cmds,
		theme:          lipgloss.NewStyle().Foreground(lipgloss.Color("#00ffff")),
		sessions:       sm,
		currentSession: currentSession,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.EnterAltScreen
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			// Save current session before quitting
			if m.currentSession != nil {
				m.sessions.Save(m.currentSession)
			}
			return m, tea.Quit
		case "enter":
			if strings.TrimSpace(m.input) == "" || m.loading {
				return m, nil
			}
			input := m.input
			m.input = ""
			m.statusMessage = ""

			// Handle session commands (:s, :l, :n, :load, :rename, :delete)
			if handled, newModel, cmd := m.handleSessionCommand(input); handled {
				return newModel, cmd
			}

			// Handle tool commands (/...)
			if cmd, args, isCmd := tools.ParseCommand(input); isCmd {
				return m, m.handleCommand(cmd, args)
			}

			// Regular message
			m.addMessage("user", input)
			m.loading = true
			return m, m.sendMessage(input)
		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		default:
			if len(msg.String()) == 1 {
				m.input += msg.String()
			}
		}
	case responseMsg:
		m.loading = false
		m.addMessage("assistant", msg.text)
	case startStreamMsg:
		m.streamText = msg.textCh
		m.streamErr = msg.errCh
		// Add empty assistant message to stream into
		m.addMessage("assistant", "")
		return m, streamNext(m.streamText, m.streamErr)
	case streamingMsg:
		if m.currentSession != nil && len(m.currentSession.Messages) > 0 {
			lastIdx := len(m.currentSession.Messages) - 1
			if content, ok := m.currentSession.Messages[lastIdx].Content.(string); ok {
				m.currentSession.Messages[lastIdx].Content = content + msg.text
			}
		}
		return m, streamNext(m.streamText, m.streamErr)
	case streamDoneMsg:
		m.loading = false
		m.streamText = nil
		m.streamErr = nil
		// Auto-save after stream completes
		if m.currentSession != nil {
			m.sessions.Save(m.currentSession)
		}
	case errorMsg:
		m.loading = false
		m.streamText = nil
		m.streamErr = nil
		m.addMessage("error", msg.err.Error())
	case statusMsg:
		m.statusMessage = msg.text
	}
	return m, nil
}

// handleSessionCommand handles session management commands
// Returns (handled, newModel, cmd)
func (m Model) handleSessionCommand(input string) (bool, tea.Model, tea.Cmd) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, ":") {
		return false, m, nil
	}

	parts := strings.Fields(input[1:]) // Remove leading ":"
	if len(parts) == 0 {
		return false, m, nil
	}

	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "s", "save":
		// :s or :save - save current session
		if m.currentSession != nil {
			if err := m.sessions.Save(m.currentSession); err != nil {
				m.statusMessage = "Save failed: " + err.Error()
			} else {
				m.statusMessage = "Session saved: " + m.currentSession.Name
			}
		}
		return true, m, nil

	case "n", "new":
		// :n or :new [name] - create new session
		name := "New Chat"
		if len(args) > 0 {
			name = strings.Join(args, " ")
		}
		// Save current before creating new
		if m.currentSession != nil {
			m.sessions.Save(m.currentSession)
		}
		m.currentSession = m.sessions.CreateSession(name, m.model)
		m.statusMessage = "New session: " + name
		return true, m, nil

	case "l", "list":
		// :l or :list - list all sessions
		infos := m.sessions.ListSessionsInfo()
		var b strings.Builder
		b.WriteString("Sessions:\n")
		for _, info := range infos {
			marker := "  "
			if info.IsCurrent {
				marker = "→ "
			}
			b.WriteString(fmt.Sprintf("%s[%s] %s (%d msgs, %s)\n",
				marker,
				info.ID[:8], // Show first 8 chars of ID
				info.Name,
				info.Messages,
				info.UpdatedAt.Format("Jan 2 15:04"),
			))
		}
		b.WriteString("\nUse :load <id> to load a session")
		m.addMessage("system", b.String())
		return true, m, nil

	case "load":
		// :load <id> - load a session by ID (partial match supported)
		if len(args) == 0 {
			m.statusMessage = "Usage: :load <session-id>"
			return true, m, nil
		}
		
		// Save current first
		if m.currentSession != nil {
			m.sessions.Save(m.currentSession)
		}

		sess, err := m.sessions.FindSessionByPartialID(args[0])
		if err != nil {
			m.statusMessage = err.Error()
			return true, m, nil
		}
		m.currentSession = sess
		m.sessions.SetCurrent(sess)
		m.statusMessage = "Loaded: " + sess.Name
		return true, m, nil

	case "rename":
		// :rename <new-name> - rename current session
		if len(args) == 0 {
			m.statusMessage = "Usage: :rename <new-name>"
			return true, m, nil
		}
		if m.currentSession == nil {
			m.statusMessage = "No current session"
			return true, m, nil
		}
		newName := strings.Join(args, " ")
		if err := m.sessions.RenameSession(m.currentSession.ID, newName); err != nil {
			m.statusMessage = "Rename failed: " + err.Error()
		} else {
			m.currentSession.Name = newName
			m.statusMessage = "Renamed to: " + newName
		}
		return true, m, nil

	case "delete", "del":
		// :delete <id> - delete a session
		if len(args) == 0 {
			m.statusMessage = "Usage: :delete <session-id>"
			return true, m, nil
		}
		sess, err := m.sessions.FindSessionByPartialID(args[0])
		if err != nil {
			m.statusMessage = err.Error()
			return true, m, nil
		}
		if m.currentSession != nil && m.currentSession.ID == sess.ID {
			m.statusMessage = "Cannot delete current session"
			return true, m, nil
		}
		if err := m.sessions.DeleteSession(sess.ID); err != nil {
			m.statusMessage = "Delete failed: " + err.Error()
		} else {
			m.statusMessage = "Deleted: " + sess.Name
		}
		return true, m, nil

	case "export":
		// :export <path> - export current session to file
		if len(args) == 0 {
			m.statusMessage = "Usage: :export <path>"
			return true, m, nil
		}
		if m.currentSession == nil {
			m.statusMessage = "No current session"
			return true, m, nil
		}
		if err := m.sessions.ExportSession(m.currentSession.ID, args[0]); err != nil {
			m.statusMessage = "Export failed: " + err.Error()
		} else {
			m.statusMessage = "Exported to: " + args[0]
		}
		return true, m, nil

	case "import":
		// :import <path> - import session from file
		if len(args) == 0 {
			m.statusMessage = "Usage: :import <path>"
			return true, m, nil
		}
		sess, err := m.sessions.ImportSession(args[0])
		if err != nil {
			m.statusMessage = "Import failed: " + err.Error()
		} else {
			m.statusMessage = "Imported: " + sess.Name
		}
		return true, m, nil

	case "name":
		// :name <new-name> - alias for :rename
		if len(args) == 0 {
			m.statusMessage = "Usage: :name <new-name>"
			return true, m, nil
		}
		if m.currentSession == nil {
			m.statusMessage = "No current session"
			return true, m, nil
		}
		newName := strings.Join(args, " ")
		if err := m.sessions.RenameSession(m.currentSession.ID, newName); err != nil {
			m.statusMessage = "Rename failed: " + err.Error()
		} else {
			m.currentSession.Name = newName
			m.statusMessage = "Session named: " + newName
		}
		return true, m, nil

	case "help", "h", "?":
		// :help - show session commands
		help := `Session Commands:
  :s, :save          Save current session
  :n, :new [name]    Create new session
  :l, :list          List all sessions
  :load <id>         Load session by ID
  :name <name>       Name current session
  :rename <name>     Rename current session (alias: :name)
  :delete <id>       Delete a session
  :export <path>     Export session to file
  :import <path>     Import session from file
  :help              Show this help`
		m.addMessage("system", help)
		return true, m, nil
	}

	return false, m, nil
}

func (m Model) addMessage(role string, content string) {
	if m.currentSession == nil {
		return
	}
	// Use session manager's AddMessage for proper auto-save tracking
	shouldSave := m.sessions.AddMessage(role, content)
	if shouldSave {
		// Auto-save triggered (every N messages)
		m.sessions.Save(m.currentSession)
	}
}

func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}
	var b strings.Builder
	b.WriteString(m.theme.Render("GOLEM"))

	// Show current session
	if m.currentSession != nil {
		b.WriteString(fmt.Sprintf(" [%s]", m.currentSession.Name))
	}
	b.WriteString("\n\n")

	// Show messages
	if m.currentSession != nil {
		for _, msg := range m.currentSession.Messages {
			prefix := "You"
			style := lipgloss.NewStyle()
			switch msg.Role {
			case "assistant":
				prefix = "Golem"
				style = m.theme
			case "system":
				prefix = "System"
				style = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
			case "error":
				prefix = "Error"
				style = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5555"))
			}
			content := msg.Content
			if s, ok := content.(string); ok {
				b.WriteString(style.Render(prefix+": ") + s + "\n\n")
			}
		}
	}

	if m.loading {
		b.WriteString("...streaming...\n\n")
	}
	b.WriteString("› " + m.input + "\n")

	// Status bar
	var statusParts []string
	if m.statusMessage != "" {
		statusParts = append(statusParts, m.statusMessage)
	}
	statusParts = append(statusParts, fmt.Sprintf("Model: %s", m.model))
	statusParts = append(statusParts, ":help for commands")

	status := " " + strings.Join(statusParts, " | ")
	b.WriteString("\n" + m.theme.Render(status))

	return b.String()
}

func (m Model) handleCommand(cmd string, args []string) tea.Cmd {
	if command, ok := m.cmds[cmd]; ok {
		return func() tea.Msg {
			output, err := command.Handler(context.Background(), args)
			if err != nil {
				return errorMsg{err: err}
			}
			if output == "EXIT" {
				return tea.Quit()
			}
			m.addMessage("user", "/"+cmd+" "+strings.Join(args, " "))
			return responseMsg{text: output}
		}
	}
	return func() tea.Msg {
		return errorMsg{err: fmt.Errorf("unknown command: %s", cmd)}
	}
}

func (m Model) sendMessage(input string) tea.Cmd {
	return func() tea.Msg {
		if m.apiKey == "" {
			return errorMsg{err: fmt.Errorf("missing API key. Set ZAI_API_KEY or ZHIPU_API_KEY")}
		}
		ctx := context.Background()

		// Build message history
		messages := []zhipu.Message{}
		if m.currentSession != nil {
			for _, msg := range m.currentSession.Messages {
				if msg.Role != "system" && msg.Role != "error" {
					messages = append(messages, msg)
				}
			}
		}

		req := &zhipu.ChatRequest{
			Model:    m.model,
			Messages: messages,
			Stream:   true,
		}
		textCh, errCh := m.client.ChatStream(ctx, req)
		return startStreamMsg{textCh: textCh, errCh: errCh}
	}
}

func streamNext(textCh <-chan string, errCh <-chan error) tea.Cmd {
	return func() tea.Msg {
		if textCh == nil || errCh == nil {
			return streamDoneMsg{}
		}
		select {
		case text, ok := <-textCh:
			if !ok {
				return streamDoneMsg{}
			}
			if text == "" {
				return streamDoneMsg{}
			}
			return streamingMsg{text: text}
		case err, ok := <-errCh:
			if ok && err != nil {
				return errorMsg{err: err}
			}
			return streamDoneMsg{}
		}
	}
}

func Run() error {
	settings, err := config.Load()
	if err != nil {
		return err
	}
	model := NewAppModel(settings)
	app := core.NewApp(model, core.WithAltScreen(), core.WithTitle("Golem"))
	return app.Run()
}
