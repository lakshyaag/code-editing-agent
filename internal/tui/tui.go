package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	"agent/internal/agent"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type (
	messageType int
	message     struct {
		mType       messageType
		content     string
		isCollapsed bool
		isError     bool
		isStreaming bool
	}
)

const (
	userMessage messageType = iota
	agentMessage
	toolMessage
	streamChunk
)

// A message to indicate that the agent has finished processing
type agentResponseMsg []agent.Message

// A message for streaming content chunks
type streamChunkMsg string

// A message for tool messages during streaming
type toolMessageMsg agent.Message

// A message for streaming completion
type streamCompleteMsg struct {
	finalMessages []agent.Message
}

type model struct {
	viewport       viewport.Model
	textarea       textarea.Model
	spinner        spinner.Model
	agent          *agent.Agent
	err            error
	showSpinner    bool
	messages       []message
	width, height  int
	showStatusBar  bool
	clickableLines map[int]int
	streamingMsg   *message // Current message being streamed
	// Channels for real-time streaming
	streamChunkChan    chan streamChunkMsg
	toolMessageChan    chan toolMessageMsg
	streamCompleteChan chan streamCompleteMsg
}

func InitialModel(agent *agent.Agent) *model {
	// Initialize text area
	ta := textarea.New()
	ta.Placeholder = "Enter your message here..."
	ta.Focus()
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	// Initialize spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	// Initialize viewport
	vp := viewport.New(80, 20)
	vp.SetContent("Welcome to the AI Agent!")

	return &model{
		textarea:           ta,
		viewport:           vp,
		spinner:            s,
		agent:              agent,
		showSpinner:        false,
		messages:           []message{{mType: agentMessage, content: "Welcome to the AI Agent!"}},
		showStatusBar:      true,
		clickableLines:     make(map[int]int),
		streamChunkChan:    make(chan streamChunkMsg, 100),
		toolMessageChan:    make(chan toolMessageMsg, 10),
		streamCompleteChan: make(chan streamCompleteMsg, 1),
	}
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		sCmd  tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)
	m.spinner, sCmd = m.spinner.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Adjust layout
		m.viewport.Width = m.width
		m.viewport.Height = m.height - m.textarea.Height() - lipgloss.Height(m.statusBarView())
		m.textarea.SetWidth(m.width)
		m.viewport.SetContent(m.renderConversation())
		return m, nil
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			// Check if click is within viewport
			if msg.Y < m.viewport.Height {
				clickedLine := m.viewport.YOffset + msg.Y
				if index, ok := m.clickableLines[clickedLine]; ok {
					m.messages[index].isCollapsed = !m.messages[index].isCollapsed
					m.viewport.SetContent(m.renderConversation())
					return m, nil
				}
			}
		}
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			userInput := m.textarea.Value()
			if userInput == "" {
				return m, nil
			}

			m.messages = append(m.messages, message{mType: userMessage, content: userInput})
			m.viewport.SetContent(m.renderConversation())
			m.textarea.Reset()
			m.showSpinner = true
			m.textarea.Blur()

			// Don't create streaming message placeholder yet - wait for actual text chunks
			// Tool messages will appear first, then streaming message when text starts

			return m, tea.Batch(sCmd, m.streamingCommand(userInput))
		}
	case streamStartMsg:
		// Start the real-time streaming process
		go func() {
			// Call the agent's ProcessMessage for streaming with tool callback
			response, err := m.agent.ProcessMessage(context.Background(), msg.userInput,
				// Text callback for streaming chunks
				func(chunk string) error {
					select {
					case m.streamChunkChan <- streamChunkMsg(chunk):
					default:
						// Channel full, skip chunk to avoid blocking
					}
					return nil
				},
				// Tool callback for immediate tool message display
				func(toolMsg agent.Message) error {
					select {
					case m.toolMessageChan <- toolMessageMsg(toolMsg):
					default:
						// Channel full, skip to avoid blocking
					}
					return nil
				})

			if err != nil {
				m.streamCompleteChan <- streamCompleteMsg{
					finalMessages: []agent.Message{
						{Type: agent.AgentMessage, Content: fmt.Sprintf("Error: %v", err), IsError: true},
					},
				}
				return
			}

			// Send completion with all messages
			m.streamCompleteChan <- streamCompleteMsg{finalMessages: response}
		}()

		// Start listening for chunks, tool messages, and completion
		return m, tea.Batch(
			waitForStreamChunk(m.streamChunkChan),
			waitForToolMessage(m.toolMessageChan),
			waitForStreamComplete(m.streamCompleteChan),
		)
	case toolMessageMsg:
		// Handle tool message immediately
		m.messages = append(m.messages, message{
			mType:       toolMessage,
			content:     msg.Content,
			isCollapsed: true,
			isError:     msg.IsError,
		})
		m.viewport.SetContent(m.renderConversation())
		m.viewport.GotoBottom()

		// Continue listening for tool messages
		return m, waitForToolMessage(m.toolMessageChan)
	case streamChunkMsg:
		// Handle streaming content chunk
		// Create streaming message if it doesn't exist yet
		if m.streamingMsg == nil {
			m.streamingMsg = &message{mType: agentMessage, content: "", isStreaming: true}
			m.messages = append(m.messages, *m.streamingMsg)
		}

		if m.streamingMsg != nil {
			m.streamingMsg.content += string(msg)
			// Update the last message in the list
			if len(m.messages) > 0 {
				m.messages[len(m.messages)-1] = *m.streamingMsg
			}
			m.viewport.SetContent(m.renderConversation())
			m.viewport.GotoBottom()
		}
		// Continue listening for more chunks
		return m, waitForStreamChunk(m.streamChunkChan)
	case streamCompleteMsg:
		// Handle streaming completion
		m.showSpinner = false
		m.textarea.Focus()

		// Finalize the streaming message
		if m.streamingMsg != nil {
			m.streamingMsg.isStreaming = false
			m.streamingMsg = nil
		}

		// Note: Tool messages were already added via the toolMessageMsg handler
		// We only need to process any remaining non-tool messages here
		for _, agentMsg := range msg.finalMessages {
			switch agentMsg.Type {
			case agent.StreamChunk:
				// Skip - these were already processed during streaming
				continue
			case agent.ToolMessage:
				// Skip - these were already processed via callback
				continue
			}
		}

		m.viewport.SetContent(m.renderConversation())
		m.viewport.GotoBottom()
		return m, nil
	case agentResponseMsg:
		m.showSpinner = false
		m.textarea.Focus()

		// If we were streaming, finalize the streaming message
		if m.streamingMsg != nil {
			m.streamingMsg.isStreaming = false
			m.streamingMsg = nil
		}

		// Remove the placeholder streaming message if it exists
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].isStreaming {
			m.messages = m.messages[:len(m.messages)-1]
		}

		// Accumulate all stream chunks into one message
		var accumulatedContent string
		var hasStreamChunks bool

		for _, agentMsg := range msg {
			switch agentMsg.Type {
			case agent.StreamChunk:
				accumulatedContent += agentMsg.Content
				hasStreamChunks = true
			case agent.ToolMessage:
				m.messages = append(m.messages, message{
					mType:       toolMessage,
					content:     agentMsg.Content,
					isCollapsed: true,
					isError:     agentMsg.IsError,
				})
			}
		}

		// Add the accumulated streaming content as a single agent message
		if hasStreamChunks {
			m.messages = append(m.messages, message{
				mType:   agentMessage,
				content: accumulatedContent,
			})
		}

		m.viewport.SetContent(m.renderConversation())
		m.viewport.GotoBottom()
		return m, nil
	case error:
		m.err = msg
		return m, nil
	}

	return m, tea.Batch(tiCmd, vpCmd, sCmd)
}

func (m *model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	var taView string
	if m.showSpinner {
		spinner := m.spinner.View() + " Agent is thinking..."
		taView = lipgloss.NewStyle().
			Width(m.width).
			Height(m.textarea.Height()).
			Align(lipgloss.Center, lipgloss.Center).
			Render(spinner)
	} else {
		taView = m.textarea.View()
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.viewport.View(),
		taView,
		m.statusBarView(),
	)
}

func wrapText(text string, width int) string {
	return lipgloss.NewStyle().Width(width).Render(text)
}

func (m *model) renderConversation() string {
	m.clickableLines = make(map[int]int)
	var lines []string
	var currentLine int

	userStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("99"))
	agentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	toolStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	errorToolStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	for i, msg := range m.messages {
		var renderedBlock string
		switch msg.mType {
		case userMessage:
			prefix := "You: "
			wrappedContent := wrapText(msg.content, m.viewport.Width-lipgloss.Width(prefix))
			renderedBlock = lipgloss.JoinHorizontal(lipgloss.Top, userStyle.Render(prefix), wrappedContent)
		case agentMessage:
			prefix := "Agent: "
			style := agentStyle
			wrappedContent := wrapText(msg.content, m.viewport.Width-lipgloss.Width(prefix))
			renderedBlock = lipgloss.JoinHorizontal(lipgloss.Top, style.Render(prefix), wrappedContent)
		case toolMessage:
			style := toolStyle
			if msg.isError {
				style = errorToolStyle
			}

			// Extract tool name from content for better display
			lines := strings.Split(msg.content, "\n")
			toolName := "Tool call"
			if len(lines) > 0 && strings.Contains(lines[0], "Tool Call:") {
				toolName = strings.TrimPrefix(lines[0], "🔧 Tool Call: ")
			}

			if msg.isCollapsed {
				header := style.Render(fmt.Sprintf("[+] %s", toolName))
				m.clickableLines[currentLine] = i
				renderedBlock = header
			} else {
				header := style.Render(fmt.Sprintf("[-] %s", toolName))
				m.clickableLines[currentLine] = i
				wrappedContent := wrapText(msg.content, m.viewport.Width)
				renderedBlock = lipgloss.JoinVertical(lipgloss.Left, header, wrappedContent)
			}
		}
		lines = append(lines, renderedBlock)
		currentLine += lipgloss.Height(renderedBlock)
	}
	return strings.Join(lines, "\n")
}

func (m *model) statusBarView() string {
	if !m.showStatusBar {
		return ""
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "n/a"
	}

	modelInfo := fmt.Sprintf("Model: %s", m.agent.Model)
	cwdInfo := fmt.Sprintf("CWD: %s", cwd)

	// Add token usage information
	tokenUsage := m.agent.GetTokenUsage()
	tokenInfo := fmt.Sprintf("Tokens: %d in, %d out, %d total",
		tokenUsage.InputTokens, tokenUsage.OutputTokens, tokenUsage.TotalTokens)

	status := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Padding(0, 1).Render(modelInfo),
		lipgloss.NewStyle().Padding(0, 1).Render(cwdInfo),
		lipgloss.NewStyle().Padding(0, 1).Render(tokenInfo),
	)

	return lipgloss.NewStyle().
		Width(m.width).
		Background(lipgloss.Color("235")).
		Foreground(lipgloss.Color("250")).
		Render(status)
}

func Start(agent *agent.Agent) {
	m := InitialModel(agent)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}

var (
	// Lipgloss styles
	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
)

// streamingCommand creates a command that starts real-time streaming
func (m model) streamingCommand(userInput string) tea.Cmd {
	return func() tea.Msg {
		return streamStartMsg{userInput: userInput}
	}
}

// waitForStreamChunk creates a command that waits for the next streaming chunk
func waitForStreamChunk(ch <-chan streamChunkMsg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

// waitForToolMessage creates a command that waits for the next tool message
func waitForToolMessage(ch <-chan toolMessageMsg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

// waitForStreamComplete creates a command that waits for streaming completion
func waitForStreamComplete(ch <-chan streamCompleteMsg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

// New message types for real-time streaming
type streamStartMsg struct {
	userInput string
}
