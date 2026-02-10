// Package agents provides specialized AI agents for Golem
package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

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

// ToolHandler executes a tool call and returns the result
type ToolHandler func(ctx context.Context, args map[string]interface{}) (string, error)

// ToolRegistry manages available tools for agents
type ToolRegistry struct {
	tools    []zhipu.Tool
	handlers map[string]ToolHandler
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools:    make([]zhipu.Tool, 0),
		handlers: make(map[string]ToolHandler),
	}
}

// Register adds a tool with its handler
func (r *ToolRegistry) Register(tool zhipu.Tool, handler ToolHandler) {
	r.tools = append(r.tools, tool)
	if tool.Function != nil {
		r.handlers[tool.Function.Name] = handler
	}
}

// Tools returns all registered tools
func (r *ToolRegistry) Tools() []zhipu.Tool {
	return r.tools
}

// Execute runs a tool by name with given arguments
func (r *ToolRegistry) Execute(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	handler, ok := r.handlers[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return handler(ctx, args)
}

// Coordinator orchestrates multiple agents
type Coordinator struct {
	client   *zhipu.Client
	agents   map[AgentType]*Agent
	registry *ToolRegistry
}

// NewCoordinator creates a new agent coordinator
func NewCoordinator(client *zhipu.Client) *Coordinator {
	return &Coordinator{
		client:   client,
		agents:   DefaultAgents(),
		registry: NewToolRegistry(),
	}
}

// RegisterTool adds a tool available to agents
func (c *Coordinator) RegisterTool(tool zhipu.Tool, handler ToolHandler) {
	c.registry.Register(tool, handler)
}

// RegisterBuiltinTools registers common tools for coding agents
func (c *Coordinator) RegisterBuiltinTools() {
	// read_file tool
	c.RegisterTool(
		zhipu.NewFunctionTool("read_file", "Read the contents of a file", zhipu.NewObjectSchema(
			map[string]*zhipu.JSONSchema{
				"path": zhipu.StringProp("Path to the file to read"),
			},
			[]string{"path"},
		)),
		func(ctx context.Context, args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			data, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			return string(data), nil
		},
	)

	// write_file tool
	c.RegisterTool(
		zhipu.NewFunctionTool("write_file", "Write content to a file", zhipu.NewObjectSchema(
			map[string]*zhipu.JSONSchema{
				"path":    zhipu.StringProp("Path to the file to write"),
				"content": zhipu.StringProp("Content to write to the file"),
			},
			[]string{"path", "content"},
		)),
		func(ctx context.Context, args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			content, _ := args["content"].(string)
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return "", err
			}
			return fmt.Sprintf("Wrote %d bytes to %s", len(content), path), nil
		},
	)

	// list_directory tool
	c.RegisterTool(
		zhipu.NewFunctionTool("list_directory", "List files in a directory", zhipu.NewObjectSchema(
			map[string]*zhipu.JSONSchema{
				"path": zhipu.StringProp("Directory path to list"),
			},
			[]string{"path"},
		)),
		func(ctx context.Context, args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			if path == "" {
				path = "."
			}
			entries, err := os.ReadDir(path)
			if err != nil {
				return "", err
			}
			var result strings.Builder
			for _, e := range entries {
				info, _ := e.Info()
				if info != nil {
					result.WriteString(fmt.Sprintf("%s\t%d\t%s\n", e.Type().String(), info.Size(), e.Name()))
				} else {
					result.WriteString(e.Name() + "\n")
				}
			}
			return result.String(), nil
		},
	)

	// run_command tool
	c.RegisterTool(
		zhipu.NewFunctionTool("run_command", "Execute a shell command", zhipu.NewObjectSchema(
			map[string]*zhipu.JSONSchema{
				"command": zhipu.StringProp("Command to execute"),
			},
			[]string{"command"},
		)),
		func(ctx context.Context, args map[string]interface{}) (string, error) {
			command, _ := args["command"].(string)
			cmd := exec.CommandContext(ctx, "sh", "-c", command)
			output, err := cmd.CombinedOutput()
			if err != nil {
				return string(output), fmt.Errorf("command failed: %w\n%s", err, output)
			}
			return string(output), nil
		},
	)
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

// RunWithTools executes a task with an agent that can use tools
// Implements the tool call loop: request → tool_calls → execute → continue
func (c *Coordinator) RunWithTools(ctx context.Context, agentType AgentType, task Task) (*Result, error) {
	agent, ok := c.agents[agentType]
	if !ok {
		return nil, fmt.Errorf("unknown agent: %s", agentType)
	}

	messages := []zhipu.Message{
		{Role: "system", Content: agent.SystemPrompt},
		{Role: "user", Content: formatTask(task)},
	}

	tools := c.registry.Tools()
	if len(tools) == 0 {
		// No tools registered, fall back to regular Run
		return c.Run(ctx, agentType, task)
	}

	var totalTokens int
	maxIterations := 10 // Prevent infinite loops

	for i := 0; i < maxIterations; i++ {
		resp, err := c.client.Chat(ctx, &zhipu.ChatRequest{
			Model:       agent.Model,
			Messages:    messages,
			Temperature: agent.Temperature,
			MaxTokens:   agent.MaxTokens,
			Tools:       tools,
			ToolChoice:  "auto",
		})
		if err != nil {
			return nil, fmt.Errorf("agent %s failed: %w", agent.Name, err)
		}

		totalTokens += resp.Usage.TotalTokens

		if len(resp.Choices) == 0 {
			return nil, fmt.Errorf("no response from agent")
		}

		choice := resp.Choices[0]
		messages = append(messages, choice.Message)

		// No tool calls = final response
		if len(choice.Message.ToolCalls) == 0 {
			content, _ := choice.Message.Content.(string)
			return &Result{
				Agent:   agentType,
				Content: content,
				Tokens:  totalTokens,
			}, nil
		}

		// Execute tool calls and add results
		for _, tc := range choice.Message.ToolCalls {
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				args = make(map[string]interface{})
			}

			result, err := c.registry.Execute(ctx, tc.Function.Name, args)
			if err != nil {
				result = fmt.Sprintf("Error: %v", err)
			}

			messages = append(messages, zhipu.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    result,
			})
		}
	}

	return nil, fmt.Errorf("agent exceeded maximum tool call iterations")
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
