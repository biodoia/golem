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
	"github.com/biodoia/golem/internal/tools"
	"github.com/biodoia/golem/pkg/zhipu"
)

type Model struct {
	input      string
	messages   []Message
	width      int
	height     int
	loading    bool
	model      string
	apiKey     string
	client     *zhipu.Client
	cmds       map[string]*tools.Command
	ready      bool
	theme      lipgloss.Style
	streamText <-chan string
	streamErr  <-chan error
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

func NewAppModel(settings config.Settings) Model {
	client := zhipu.NewClient(settings.APIKey)
	cmds := tools.Commands()
	extCmds := tools.LoadExternalCommands(config.CommandsSearchPaths(settings.CommandsPath))
	for k, v := range extCmds {
		cmds[k] = v
	}
	return Model{
		model:  settings.Model,
		apiKey: settings.APIKey,
		client: client,
		cmds:   cmds,
		theme:  lipgloss.NewStyle().Foreground(lipgloss.Color("#00ffff")),
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
			m.messages = append(m.messages, Message{Role: "user", Content: input, Time: time.Now()})
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
		m.messages = append(m.messages, Message{Role: "assistant", Content: msg.text, Time: time.Now()})
	case startStreamMsg:
		m.streamText = msg.textCh
		m.streamErr = msg.errCh
		return m, streamNext(m.streamText, m.streamErr)
	case streamingMsg:
		if len(m.messages) == 0 || m.messages[len(m.messages)-1].Role != "assistant" {
			m.messages = append(m.messages, Message{Role: "assistant", Content: msg.text, Time: time.Now()})
		} else {
			m.messages[len(m.messages)-1].Content += msg.text
		}
		return m, streamNext(m.streamText, m.streamErr)
	case streamDoneMsg:
		m.loading = false
		m.streamText = nil
		m.streamErr = nil
	case errorMsg:
		m.loading = false
		m.streamText = nil
		m.streamErr = nil
		m.messages = append(m.messages, Message{Role: "error", Content: msg.err.Error(), Time: time.Now()})
	}
	return m, nil
}

func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}
	var b strings.Builder
	b.WriteString(m.theme.Render("GOLEM") + "\n\n")
	for _, msg := range m.messages {
		prefix := "You"
		switch msg.Role {
		case "assistant":
			prefix = "Golem"
		case "error":
			prefix = "Error"
		}
		b.WriteString(fmt.Sprintf("%s: %s\n\n", prefix, msg.Content))
	}
	if m.loading {
		b.WriteString("...thinking...\n\n")
	}
	b.WriteString("› " + m.input + "\n")
	b.WriteString(renderStatus(m.width, m.model))
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
		req := &zhipu.ChatRequest{
			Model:    m.model,
			Messages: []zhipu.Message{{Role: "user", Content: input}},
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
