package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/lipgloss"
)

// Crush-style color palette
var (
	// Primary gradient (purple → cyan)
	colorPrimary   = lipgloss.Color("#8B5CF6")
	colorSecondary = lipgloss.Color("#06B6D4")
	colorAccent    = lipgloss.Color("#F59E0B")
	colorSuccess   = lipgloss.Color("#10B981")
	colorError     = lipgloss.Color("#EF4444")
	colorMuted     = lipgloss.Color("#6B7280")
	
	// Styles
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPrimary).
		MarginBottom(1)
	
	promptStyle = lipgloss.NewStyle().
		Foreground(colorSecondary).
		Bold(true)
	
	responseStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E5E7EB"))
	
	statusStyle = lipgloss.NewStyle().
		Foreground(colorMuted).
		Italic(true)
	
	errorStyle = lipgloss.NewStyle().
		Foreground(colorError).
		Bold(true)
)

// ASCII Art Logo
const logo = `
   ██████╗  ██████╗ ██╗     ███████╗███╗   ███╗
  ██╔════╝ ██╔═══██╗██║     ██╔════╝████╗ ████║
  ██║  ███╗██║   ██║██║     █████╗  ██╔████╔██║
  ██║   ██║██║   ██║██║     ██╔══╝  ██║╚██╔╝██║
  ╚██████╔╝╚██████╔╝███████╗███████╗██║ ╚═╝ ██║
   ╚═════╝  ╚═════╝ ╚══════╝╚══════╝╚═╝     ╚═╝
`

// Animation frames for loading
var loadingFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Model represents the TUI state
type Model struct {
	// Input
	input    string
	cursor   int
	
	// Output
	messages []Message
	
	// Animation
	spring       harmonica.Spring
	logoOffset   float64
	logoVelocity float64
	loadingFrame int
	isLoading    bool
	
	// State
	width    int
	height   int
	model    string
	provider string
	ready    bool
	quitting bool
	
	// Intro animation
	introPhase int
	introTick  int
}

// Message in conversation
type Message struct {
	Role    string
	Content string
	Time    time.Time
}

// Messages
type tickMsg time.Time
type responseMsg string
type errorMsg error

func initialModel() Model {
	// Initialize harmonica spring for smooth animations
	spring := harmonica.NewSpring(harmonica.FPS(60), 6.0, 0.5)
	
	return Model{
		spring:   spring,
		model:    "glm-4-32b-0414",
		provider: "zhipu",
		messages: []Message{},
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tick(),
		tea.EnterAltScreen,
	)
}

func tick() tea.Cmd {
	return tea.Tick(time.Second/60, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			if m.input != "" && !m.isLoading {
				// Add user message
				m.messages = append(m.messages, Message{
					Role:    "user",
					Content: m.input,
					Time:    time.Now(),
				})
				input := m.input
				m.input = ""
				m.isLoading = true
				return m, sendMessage(input, m.model)
			}
		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		default:
			if len(msg.String()) == 1 {
				m.input += msg.String()
			}
		}
	
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
	
	case tickMsg:
		// Update animations
		m.introTick++
		if m.introTick > 10 && m.introPhase < 5 {
			m.introPhase++
			m.introTick = 0
		}
		
		// Spring animation for logo
		targetOffset := 0.0
		m.logoOffset, m.logoVelocity = m.spring.Update(m.logoOffset, m.logoVelocity, targetOffset)
		
		// Loading animation
		if m.isLoading {
			m.loadingFrame = (m.loadingFrame + 1) % len(loadingFrames)
		}
		
		return m, tick()
	
	case responseMsg:
		m.isLoading = false
		m.messages = append(m.messages, Message{
			Role:    "assistant",
			Content: string(msg),
			Time:    time.Now(),
		})
	
	case errorMsg:
		m.isLoading = false
		m.messages = append(m.messages, Message{
			Role:    "error",
			Content: msg.Error(),
			Time:    time.Now(),
		})
	}
	
	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	
	if !m.ready {
		return "Loading..."
	}
	
	var b strings.Builder
	
	// Animated logo with gradient
	if m.introPhase >= 1 {
		logoLines := strings.Split(logo, "\n")
		for i, line := range logoLines {
			// Gradient from purple to cyan
			ratio := float64(i) / float64(len(logoLines))
			color := lerpColor(colorPrimary, colorSecondary, ratio)
			style := lipgloss.NewStyle().Foreground(color)
			b.WriteString(style.Render(line) + "\n")
		}
	}
	
	// Tagline
	if m.introPhase >= 2 {
		tagline := statusStyle.Render("GLM-Powered CLI Coding Agent • Z.AI Native")
		b.WriteString(tagline + "\n\n")
	}
	
	// Model info
	if m.introPhase >= 3 {
		modelInfo := fmt.Sprintf("Model: %s • Provider: %s",
			lipgloss.NewStyle().Foreground(colorAccent).Render(m.model),
			lipgloss.NewStyle().Foreground(colorSuccess).Render(m.provider),
		)
		b.WriteString(modelInfo + "\n\n")
	}
	
	// Messages
	if m.introPhase >= 4 {
		for _, msg := range m.messages {
			switch msg.Role {
			case "user":
				b.WriteString(promptStyle.Render("You: "))
				b.WriteString(msg.Content + "\n\n")
			case "assistant":
				b.WriteString(lipgloss.NewStyle().Foreground(colorSuccess).Bold(true).Render("Golem: "))
				b.WriteString(responseStyle.Render(msg.Content) + "\n\n")
			case "error":
				b.WriteString(errorStyle.Render("Error: " + msg.Content) + "\n\n")
			}
		}
		
		// Loading indicator
		if m.isLoading {
			frame := loadingFrames[m.loadingFrame]
			b.WriteString(lipgloss.NewStyle().Foreground(colorSecondary).Render(frame + " Thinking...") + "\n\n")
		}
		
		// Input prompt
		prompt := promptStyle.Render("› ")
		cursor := ""
		if m.introTick%20 < 10 {
			cursor = "█"
		}
		b.WriteString(prompt + m.input + cursor + "\n")
	}
	
	// Status bar
	if m.introPhase >= 5 {
		statusBar := lipgloss.NewStyle().
			Width(m.width).
			Background(lipgloss.Color("#1F2937")).
			Foreground(colorMuted).
			Padding(0, 1).
			Render(fmt.Sprintf(" GOLEM v0.1.0 │ %s │ Press Ctrl+C to quit", time.Now().Format("15:04")))
		
		// Position at bottom
		padding := m.height - strings.Count(b.String(), "\n") - 2
		if padding > 0 {
			b.WriteString(strings.Repeat("\n", padding))
		}
		b.WriteString(statusBar)
	}
	
	return b.String()
}

// lerpColor interpolates between two colors
func lerpColor(a, b lipgloss.Color, t float64) lipgloss.Color {
	// Simple approach: just return based on threshold
	if t < 0.5 {
		return a
	}
	return b
}

// sendMessage sends a message to the AI
func sendMessage(input string, model string) tea.Cmd {
	return func() tea.Msg {
		// TODO: Implement actual Z.AI call
		// For now, simulate response
		time.Sleep(500 * time.Millisecond)
		
		if strings.Contains(strings.ToLower(input), "hello") {
			return responseMsg("你好！我是 Golem，基于 GLM 的智能编程助手。有什么我可以帮你的吗？")
		}
		
		return responseMsg(fmt.Sprintf("收到你的消息: %q\n\n我正在使用 %s 模型处理你的请求...", input, model))
	}
}

func main() {
	// Check for one-shot mode
	if len(os.Args) > 1 && os.Args[1] != "" && !strings.HasPrefix(os.Args[1], "-") {
		// One-shot query mode
		query := strings.Join(os.Args[1:], " ")
		fmt.Printf("Query: %s\n", query)
		fmt.Println("(One-shot mode - implement with zhipu client)")
		return
	}
	
	// Interactive TUI mode
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
