// Package agents provides specialized AI agents for Golem
package agents

import (
	"context"
	"fmt"

	"github.com/biodoia/golem/pkg/zhipu"
)

// AgentType defines the type of agent
type AgentType string

const (
	AgentArchitect AgentType = "architect"
	AgentCoder     AgentType = "coder"
	AgentReviewer  AgentType = "reviewer"
	AgentDebugger  AgentType = "debugger"
	AgentTester    AgentType = "tester"
	AgentDocs      AgentType = "docs"
)

// Agent represents a specialized AI agent
type Agent struct {
	Type        AgentType
	Name        string
	Description string
	Model       string
	SystemPrompt string
	Temperature float64
	MaxTokens   int
}

// DefaultAgents returns pre-configured specialized agents
func DefaultAgents() map[AgentType]*Agent {
	return map[AgentType]*Agent{
		AgentArchitect: {
			Type:        AgentArchitect,
			Name:        "Architect",
			Description: "Plans system architecture and designs solutions",
			Model:       zhipu.ModelGLMZ1_32B, // Deep thinking for architecture
			SystemPrompt: `You are a senior software architect. Your role is to:
- Analyze requirements and propose system designs
- Break down complex problems into manageable components
- Define interfaces and contracts between modules
- Consider scalability, maintainability, and performance
- Create high-level plans before implementation

Always think step by step. Output structured plans with clear milestones.`,
			Temperature: 0.3,
			MaxTokens:   4096,
		},

		AgentCoder: {
			Type:        AgentCoder,
			Name:        "Coder",
			Description: "Writes and implements code",
			Model:       zhipu.ModelGLM4_32B, // Fast, good at code
			SystemPrompt: `You are an expert programmer. Your role is to:
- Write clean, efficient, well-documented code
- Follow best practices and coding standards
- Implement solutions based on architectural plans
- Handle edge cases and errors properly
- Use appropriate design patterns

Output code with clear comments. Prefer simplicity over cleverness.`,
			Temperature: 0.2,
			MaxTokens:   8192,
		},

		AgentReviewer: {
			Type:        AgentReviewer,
			Name:        "Reviewer",
			Description: "Reviews code for quality and issues",
			Model:       zhipu.ModelGLMZ1_32B, // Deep analysis
			SystemPrompt: `You are a meticulous code reviewer. Your role is to:
- Check for bugs, security issues, and logic errors
- Verify code follows best practices and standards
- Suggest improvements for readability and performance
- Ensure proper error handling and edge cases
- Validate against requirements and specifications

Be thorough but constructive. Prioritize issues by severity.`,
			Temperature: 0.1,
			MaxTokens:   4096,
		},

		AgentDebugger: {
			Type:        AgentDebugger,
			Name:        "Debugger",
			Description: "Diagnoses and fixes bugs",
			Model:       zhipu.ModelGLMZ1_32B, // Reasoning for debugging
			SystemPrompt: `You are a debugging expert. Your role is to:
- Analyze error messages and stack traces
- Identify root causes of bugs
- Propose and verify fixes
- Explain why bugs occurred
- Suggest preventive measures

Think systematically. Reproduce → Diagnose → Fix → Verify.`,
			Temperature: 0.2,
			MaxTokens:   4096,
		},

		AgentTester: {
			Type:        AgentTester,
			Name:        "Tester",
			Description: "Writes tests and validates code",
			Model:       zhipu.ModelGLM4_32B,
			SystemPrompt: `You are a QA engineer and test writer. Your role is to:
- Write comprehensive unit tests
- Create integration and e2e tests
- Identify edge cases to test
- Ensure good test coverage
- Write clear test descriptions

Follow testing best practices. Test behavior, not implementation.`,
			Temperature: 0.2,
			MaxTokens:   4096,
		},

		AgentDocs: {
			Type:        AgentDocs,
			Name:        "Docs",
			Description: "Writes documentation and comments",
			Model:       zhipu.ModelGLM4_32B,
			SystemPrompt: `You are a technical writer. Your role is to:
- Write clear, comprehensive documentation
- Create README files and guides
- Document APIs and interfaces
- Add helpful code comments
- Explain complex concepts simply

Focus on clarity and completeness. Include examples.`,
			Temperature: 0.4,
			MaxTokens:   4096,
		},
	}
}

// Coordinator orchestrates multiple agents
type Coordinator struct {
	client *zhipu.Client
	agents map[AgentType]*Agent
}

// NewCoordinator creates a new agent coordinator
func NewCoordinator(client *zhipu.Client) *Coordinator {
	return &Coordinator{
		client: client,
		agents: DefaultAgents(),
	}
}

// Task represents a task for agents
type Task struct {
	Description string
	Context     string
	Files       []string
}

// Result represents an agent's output
type Result struct {
	Agent   AgentType
	Content string
	Tokens  int
}

// Run executes a task with a specific agent
func (c *Coordinator) Run(ctx context.Context, agentType AgentType, task Task) (*Result, error) {
	agent, ok := c.agents[agentType]
	if !ok {
		return nil, fmt.Errorf("unknown agent: %s", agentType)
	}

	messages := []zhipu.Message{
		{Role: "system", Content: agent.SystemPrompt},
		{Role: "user", Content: formatTask(task)},
	}

	resp, err := c.client.Chat(ctx, &zhipu.ChatRequest{
		Model:       agent.Model,
		Messages:    messages,
		Temperature: agent.Temperature,
		MaxTokens:   agent.MaxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("agent %s failed: %w", agent.Name, err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from agent")
	}

	content, _ := resp.Choices[0].Message.Content.(string)
	return &Result{
		Agent:   agentType,
		Content: content,
		Tokens:  resp.Usage.TotalTokens,
	}, nil
}

// Plan executes a multi-agent workflow
func (c *Coordinator) Plan(ctx context.Context, task Task) ([]Result, error) {
	results := make([]Result, 0)

	// 1. Architect plans
	archResult, err := c.Run(ctx, AgentArchitect, task)
	if err != nil {
		return nil, fmt.Errorf("architect: %w", err)
	}
	results = append(results, *archResult)

	// 2. Coder implements
	coderTask := Task{
		Description: task.Description,
		Context:     archResult.Content, // Use architect's plan
		Files:       task.Files,
	}
	coderResult, err := c.Run(ctx, AgentCoder, coderTask)
	if err != nil {
		return nil, fmt.Errorf("coder: %w", err)
	}
	results = append(results, *coderResult)

	// 3. Reviewer checks
	reviewTask := Task{
		Description: "Review the following implementation",
		Context:     coderResult.Content,
		Files:       task.Files,
	}
	reviewResult, err := c.Run(ctx, AgentReviewer, reviewTask)
	if err != nil {
		return nil, fmt.Errorf("reviewer: %w", err)
	}
	results = append(results, *reviewResult)

	return results, nil
}

// formatTask formats a task for the agent
func formatTask(task Task) string {
	result := task.Description
	if task.Context != "" {
		result += "\n\n## Context\n" + task.Context
	}
	if len(task.Files) > 0 {
		result += "\n\n## Files\n"
		for _, f := range task.Files {
			result += "- " + f + "\n"
		}
	}
	return result
}

// GetAgent returns an agent by type
func (c *Coordinator) GetAgent(t AgentType) (*Agent, bool) {
	a, ok := c.agents[t]
	return a, ok
}

// ListAgents returns all available agents
func (c *Coordinator) ListAgents() []*Agent {
	agents := make([]*Agent, 0, len(c.agents))
	for _, a := range c.agents {
		agents = append(agents, a)
	}
	return agents
}
