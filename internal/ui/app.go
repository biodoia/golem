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
	input         string
	width         int
	height        int
	loading       bool
	model         string
	apiKey        string
	client        *zhipu.Client
	cmds          map[string]*tools.Command
	ready         bool
	theme         lipgloss.Style
	streamText    <-chan string
	streamErr     <-chan error
	sessions      *session.SessionManager
	currentSession *session.Session
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
		model:         settings.Model,
		apiKey:        settings.APIKey,
		client:        client,
		cmds:          cmds,
		theme:         lipgloss.NewStyle().Foreground(lipgloss.Color("#00ffff")),
		sessions:      sm,
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
			if cmd, args, isCmd := tools.ParseCommand(input); isCmd {
				return m, m.handleCommand(cmd, args)
			}
			m.addMessage("user", input)
			m.loading = true
			return m, m.sendMessage(input)
		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		case ":s":
			// Save session
			if m.currentSession != nil {
				m.sessions.Save(m.currentSession)
				m.addMessage("system", "Session saved: "+m.currentSession.Name)
			}
		case ":n":
			// New session
			m.currentSession = m.sessions.CreateSession("New Chat", m.model)
			m.addMessage("system", "New session started")
		case ":l":
			// List sessions
			sessions := m.sessions.ListSessions()
			var b strings.Builder
			b.WriteString("Sessions:\n")
			for _, s := range sessions {
				b.WriteString(fmt.Sprintf("  - %s (%s)\n", s.Name, s.ID))
			}
			m.addMessage("assistant", b.String())
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
		return m, streamNext(m.streamText, m.streamErr)
	case streamingMsg:
		if m.currentSession == nil || len(m.currentSession.Messages) == 0 {
			m.addMessage("assistant", msg.text)
		} else {
			lastIdx := len(m.currentSession.Messages) - 1
			m.currentSession.Messages[lastIdx].Content = m.currentSession.Messages[lastIdx].Content.(string) + msg.text
		}
		return m, streamNext(m.streamText, m.streamErr)
	case streamDoneMsg:
		m.loading = false
		m.streamText = nil
		m.streamErr = nil
		// Auto-save on stream done
		if m.currentSession != nil {
			m.sessions.Save(m.currentSession)
		}
	case errorMsg:
		m.loading = false
		m.streamText = nil
		m.streamErr = nil
		m.addMessage("error", msg.err.Error())
	}
	return m, nil
}

func (m Model) addMessage(role string, content string) {
	if m.currentSession == nil {
		return
	}
	m.currentSession.Messages = append(m.currentSession.Messages, zhipu.Message{
		Role:    role,
		Content: content,
	})
}

func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}
	var b strings.Builder
	b.WriteString(m.theme.Render("GOLEM") + "\n")

	// Show current session
	if m.currentSession != nil {
		b.WriteString(fmt.Sprintf(" [%s]\n", m.currentSession.Name))
	}
	b.WriteString("\n")

	// Show messages
	if m.currentSession != nil {
		for _, msg := range m.currentSession.Messages {
			prefix := "You"
			switch msg.Role {
			case "assistant":
				prefix = "Golem"
			case "system":
				prefix = "System"
			case "error":
				prefix = "Error"
			}
			content := msg.Content
			if s, ok := content.(string); ok {
				b.WriteString(fmt.Sprintf("%s: %s\n\n", prefix, s))
			}
		}
	}

	if m.loading {
		b.WriteString("...streaming...\n\n")
	}
	b.WriteString("› " + m.input + "\n")

	// Status bar
	status := fmt.Sprintf(" [:%s] Save | [:n] New | [:l] List | [q] Quit | Model: %s",
		m.currentSessionName(), m.model)
	b.WriteString("\n" + m.theme.Render(status))

	return b.String()
}

func (m Model) currentSessionName() string {
	if m.currentSession == nil {
		return "?"
	}
	return m.currentSession.Name
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
		messages = append(messages, zhipu.Message{Role: "user", Content: input})

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
