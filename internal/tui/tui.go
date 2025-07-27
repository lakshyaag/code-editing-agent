package tui

import (
	"context"
	"fmt"
	"os"

	"agent/internal/agent"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
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

// A message for streaming content chunks
type streamChunkMsg string

// A message for tool messages during streaming
type toolMessageMsg agent.Message

// A message for streaming completion
type streamCompleteMsg struct {
	finalMessages []agent.Message
}

type model struct {
	viewport          viewport.Model
	textarea          textarea.Model
	spinner           spinner.Model
	agent             *agent.Agent
	err               error
	showSpinner       bool
	messages          []message
	width, height     int
	showStatusBar     bool
	clickableLines    map[int]int
	streamingMsg      *message // Current message being streamed
	streamingMsgIndex int      // Index of the streaming message in the messages slice
	// Channels for real-time streaming
	streamChunkChan    chan streamChunkMsg
	toolMessageChan    chan toolMessageMsg
	streamCompleteChan chan streamCompleteMsg
	// Markdown renderer for agent messages
	markdownRenderer *glamour.TermRenderer
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

	// Initialize markdown renderer with auto-style (dark/light) and appropriate width
	markdownRenderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(78), // Slightly less than viewport width for padding
	)
	if err != nil {
		// Fallback to a simple renderer if there's an error
		markdownRenderer, _ = glamour.NewTermRenderer()
	}

	return &model{
		textarea:           ta,
		viewport:           vp,
		spinner:            s,
		agent:              agent,
		showSpinner:        false,
		messages:           []message{{mType: agentMessage, content: "Welcome to the AI Agent!"}},
		showStatusBar:      true,
		clickableLines:     make(map[int]int),
		streamingMsgIndex:  -1, // Initialize to -1 (no streaming message)
		streamChunkChan:    make(chan streamChunkMsg, 100),
		toolMessageChan:    make(chan toolMessageMsg, 10),
		streamCompleteChan: make(chan streamCompleteMsg, 1),
		markdownRenderer:   markdownRenderer,
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

		// Update markdown renderer width to match viewport width
		if m.markdownRenderer != nil {
			newRenderer, err := glamour.NewTermRenderer(
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(m.width-8), // Account for "Agent: " prefix and padding
			)
			if err == nil {
				m.markdownRenderer = newRenderer
			}
		}

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
		case tea.KeyCtrlT:
			// Unify the state of all tool messages
			var anyExpanded bool
			for _, msg := range m.messages {
				if msg.mType == toolMessage && !msg.isCollapsed {
					anyExpanded = true
					break
				}
			}

			// If any are expanded, collapse all. Otherwise, expand all.
			for i, msg := range m.messages {
				if msg.mType == toolMessage {
					m.messages[i].isCollapsed = anyExpanded
				}
			}
			m.viewport.SetContent(m.renderConversation())
			return m, nil
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
		newToolMsg := message{
			mType:       toolMessage,
			content:     msg.Content,
			isCollapsed: true,
			isError:     msg.IsError,
		}

		// If streaming has started, insert the tool message before the streaming message
		if m.streamingMsgIndex != -1 {
			// Insert at the correct position
			m.messages = append(m.messages[:m.streamingMsgIndex], append([]message{newToolMsg}, m.messages[m.streamingMsgIndex:]...)...)
			// Update the index of the streaming message
			m.streamingMsgIndex++
		} else {
			// Otherwise, just append
			m.messages = append(m.messages, newToolMsg)
		}

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
			m.streamingMsgIndex = len(m.messages) - 1 // Store the actual index
		}

		if m.streamingMsg != nil {
			m.streamingMsg.content += string(msg)
			// Update the streaming message at its tracked index
			if m.streamingMsgIndex < len(m.messages) {
				m.messages[m.streamingMsgIndex] = *m.streamingMsg
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
			m.streamingMsgIndex = -1 // Reset the index
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

func Start(agent *agent.Agent) {
	m := InitialModel(agent)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}

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
