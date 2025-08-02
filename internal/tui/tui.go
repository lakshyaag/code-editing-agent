package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"agent/internal/agent"
	"agent/internal/config"

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
	thoughtMessage
	welcomeMessage
)

// UIState groups UI-related state
type UIState struct {
	viewport       viewport.Model
	textarea       textarea.Model
	spinner        spinner.Model
	width, height  int
	showSpinner    bool
	showStatusBar  bool
	clickableLines map[int]int
	
	// Modal states
	modelSelectionMode   bool
	selectedModelIndex   int
	toolConfirmationMode bool
	toolConfirmationName string
	toolConfirmationArgs map[string]interface{}
}

// StreamState groups streaming-related state
type StreamState struct {
	streamingMsg            *message
	streamingMsgIndex       int
	streamingWasInterrupted bool
	
	// Channels
	streamChunkChan    chan streamChunkMsg
	toolMessageChan    chan toolMessageMsg
	thoughtMessageChan chan thoughtMessageMsg
	streamCompleteChan chan streamCompleteMsg
	toolConfirmationChan     chan toolConfirmationRequestMsg
	confirmationResponseChan chan bool
}

// AppConfig groups application configuration
type AppConfig struct {
	agent               *agent.Agent
	availableModels     []string
	markdownRenderer    *glamour.TermRenderer
	requireToolConfirmation bool
	enableThinkingMode      bool
}

// model represents the main application model
type model struct {
	ui       UIState
	stream   StreamState
	config   AppConfig
	messages []message
	err      error
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

	// Initialize viewport with reasonable defaults
	vp := viewport.New(80, 20)

	// Initialize markdown renderer with auto-style (dark/light) and appropriate width
	markdownRenderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(78), // Slightly less than viewport width for padding
	)
	if err != nil {
		// Fallback to a simple renderer if there's an error
		markdownRenderer, _ = glamour.NewTermRenderer()
	}

	// Available Gemini models based on the documentation
	availableModels := []string{
		"gemini-2.5-pro",
		"gemini-2.5-flash",
		"gemini-2.5-flash-lite",
		"gemini-2.0-flash",
		"gemini-2.0-flash-lite",
		"gemini-1.5-pro",
		"gemini-1.5-flash",
	}

	// Find current model index
	currentModelIndex := 1 // Default to gemini-2.5-flash
	for i, model := range availableModels {
		if model == agent.Model {
			currentModelIndex = i
			break
		}
	}

	// Load user preferences
	prefs, _ := config.LoadPreferences()
	requireConfirmation := true // Default to true
	enableThinking := false     // Default to false
	if prefs != nil {
		requireConfirmation = prefs.RequireToolConfirmation
		enableThinking = prefs.EnableThinkingMode
	}

	m := &model{
		ui: UIState{
			textarea:           ta,
			viewport:           vp,
			spinner:            s,
			showSpinner:        false,
			showStatusBar:      true,
			clickableLines:     make(map[int]int),
			modelSelectionMode: false,
			selectedModelIndex: currentModelIndex,
			width:              80,
			height:             24,
			toolConfirmationMode: false,
		},
		stream: StreamState{
			streamingMsgIndex:        -1,
			streamingWasInterrupted:  false,
			streamChunkChan:          make(chan streamChunkMsg, 100),
			toolMessageChan:          make(chan toolMessageMsg, 10),
			thoughtMessageChan:       make(chan thoughtMessageMsg, 10),
			streamCompleteChan:       make(chan streamCompleteMsg, 1),
			toolConfirmationChan:     make(chan toolConfirmationRequestMsg, 1),
			confirmationResponseChan: make(chan bool, 1),
		},
		config: AppConfig{
			agent:                   agent,
			availableModels:         availableModels,
			markdownRenderer:        markdownRenderer,
			requireToolConfirmation: requireConfirmation,
			enableThinkingMode:      enableThinking,
		},
		messages: []message{
			{mType: welcomeMessage, content: fmt.Sprintf(config.WelcomeMessage, len(config.SystemPrompt))},
		},
	}

	// Set initial content
	m.ui.viewport.SetContent(m.renderConversation())

	return m
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.ui.spinner.Tick,
		tea.WindowSize(), // Request initial window size
	)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		sCmd  tea.Cmd
	)

	// Update sub-components
	m.ui.textarea, tiCmd = m.ui.textarea.Update(msg)
	m.ui.viewport, vpCmd = m.ui.viewport.Update(msg)
	m.ui.spinner, sCmd = m.ui.spinner.Update(msg)

	// Handle different message types
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m, m.handleWindowResize(msg)
	case tea.MouseMsg:
		return m, m.handleMouseClick(msg)
	case tea.KeyMsg:
		return m, m.handleKeyPress(msg)
	case streamStartMsg:
		return m, m.handleStreamStart(msg)
	case toolMessageMsg:
		return m, m.handleToolMessage(msg)
	case thoughtMessageMsg:
		return m, m.handleThoughtMessage(msg)
	case streamChunkMsg:
		return m, m.handleStreamChunk(msg)
	case streamCompleteMsg:
		return m, m.handleStreamComplete(msg)
	case toolConfirmationRequestMsg:
		return m, m.handleToolConfirmationRequest(msg)
	case error:
		m.err = msg
		return m, nil
	}

	return m, tea.Batch(tiCmd, vpCmd, sCmd)
}

// handleWindowResize handles window resize events
func (m *model) handleWindowResize(msg tea.WindowSizeMsg) tea.Cmd {
	m.ui.width = msg.Width
	m.ui.height = msg.Height
	// Adjust layout
	m.ui.viewport.Width = m.ui.width
	m.ui.viewport.Height = m.ui.height - m.ui.textarea.Height() - lipgloss.Height(m.statusBarView())
	m.ui.textarea.SetWidth(m.ui.width)

	// Update markdown renderer width to match viewport width
	if m.config.markdownRenderer != nil {
		newRenderer, err := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(m.ui.width-8), // Account for "Agent: " prefix and padding
		)
		if err == nil {
			m.config.markdownRenderer = newRenderer
		}
	}

	m.ui.viewport.SetContent(m.renderConversation())
	return nil
}

// handleMouseClick handles mouse click events
func (m *model) handleMouseClick(msg tea.MouseMsg) tea.Cmd {
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		// Check if click is within viewport
		if msg.Y < m.ui.viewport.Height {
			clickedLine := m.ui.viewport.YOffset + msg.Y
			if index, ok := m.ui.clickableLines[clickedLine]; ok {
				m.messages[index].isCollapsed = !m.messages[index].isCollapsed
				m.ui.viewport.SetContent(m.renderConversation())
			}
		}
	}
	return nil
}

// handleKeyPress handles keyboard input
func (m *model) handleKeyPress(msg tea.KeyMsg) tea.Cmd {
	// Tool confirmation mode has highest priority
	if m.ui.toolConfirmationMode {
		return m.handleToolConfirmationKey(msg)
	}

	// Model selection mode has priority
	if m.ui.modelSelectionMode {
		return m.handleModelSelectionKey(msg)
	}

	// Handle normal mode keys
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		return tea.Quit
	case tea.KeyF2:
		return m.toggleModelSelection()
	case tea.KeyF3:
		return m.toggleToolConfirmation()
	case tea.KeyF4:
		return m.toggleThinkingMode()
	case tea.KeyCtrlT:
		return m.toggleCollapsedMessages()
	case tea.KeyEnter:
		return m.handleUserInput()
	}

	return nil
}

// handleToolConfirmationKey handles keys in tool confirmation mode
func (m *model) handleToolConfirmationKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "y", "Y":
		// User confirmed
		m.stream.confirmationResponseChan <- true
		m.ui.toolConfirmationMode = false
		m.ui.textarea.Focus()
	case "n", "N", "esc":
		// User denied
		m.stream.confirmationResponseChan <- false
		m.ui.toolConfirmationMode = false
		m.ui.textarea.Focus()
	}
	return nil
}

// handleModelSelectionKey handles keys in model selection mode
func (m *model) handleModelSelectionKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEsc:
		m.ui.modelSelectionMode = false
		m.ui.textarea.Focus()
		return nil
	case tea.KeyUp:
		if m.ui.selectedModelIndex > 0 {
			m.ui.selectedModelIndex--
		}
		return nil
	case tea.KeyDown:
		if m.ui.selectedModelIndex < len(m.config.availableModels)-1 {
			m.ui.selectedModelIndex++
		}
		return nil
	case tea.KeyEnter:
		return m.selectModel()
	}
	return nil
}

// toggleModelSelection toggles model selection mode
func (m *model) toggleModelSelection() tea.Cmd {
	m.ui.modelSelectionMode = !m.ui.modelSelectionMode
	if m.ui.modelSelectionMode {
		m.ui.textarea.Blur()
	} else {
		m.ui.textarea.Focus()
	}
	return nil
}

// toggleToolConfirmation toggles tool confirmation requirement
func (m *model) toggleToolConfirmation() tea.Cmd {
	m.config.requireToolConfirmation = !m.config.requireToolConfirmation

	// Save preference
	prefs, _ := config.LoadPreferences()
	if prefs == nil {
		prefs = &config.UserPreferences{}
	}
	prefs.RequireToolConfirmation = m.config.requireToolConfirmation
	config.SavePreferences(prefs)

	// Show feedback message
	confirmStatus := "enabled"
	if !m.config.requireToolConfirmation {
		confirmStatus = "disabled"
	}
	m.messages = append(m.messages, message{
		mType:   agentMessage,
		content: fmt.Sprintf("Tool confirmation %s", confirmStatus),
	})
	m.ui.viewport.SetContent(m.renderConversation())
	m.ui.viewport.GotoBottom()
	return nil
}

// toggleThinkingMode toggles thinking mode
func (m *model) toggleThinkingMode() tea.Cmd {
	m.config.enableThinkingMode = !m.config.enableThinkingMode

	// Save preference
	prefs, _ := config.LoadPreferences()
	if prefs == nil {
		prefs = &config.UserPreferences{}
	}
	prefs.EnableThinkingMode = m.config.enableThinkingMode
	config.SavePreferences(prefs)

	// Show feedback message
	thinkingStatus := "enabled"
	icon := "🧠"
	if !m.config.enableThinkingMode {
		thinkingStatus = "disabled"
		icon = "💭"
	}
	m.messages = append(m.messages, message{
		mType:   agentMessage,
		content: fmt.Sprintf("%s Thinking mode %s", icon, thinkingStatus),
	})
	m.ui.viewport.SetContent(m.renderConversation())
	m.ui.viewport.GotoBottom()
	return nil
}

// toggleCollapsedMessages toggles collapsed state of tool and thought messages
func (m *model) toggleCollapsedMessages() tea.Cmd {
	var anyExpanded bool
	for _, msg := range m.messages {
		if (msg.mType == toolMessage || msg.mType == thoughtMessage) && !msg.isCollapsed {
			anyExpanded = true
			break
		}
	}

	// If any are expanded, collapse all. Otherwise, expand all.
	for i, msg := range m.messages {
		if msg.mType == toolMessage || msg.mType == thoughtMessage {
			m.messages[i].isCollapsed = anyExpanded
		}
	}
	m.ui.viewport.SetContent(m.renderConversation())
	return nil
}

// handleUserInput processes user input
func (m *model) handleUserInput() tea.Cmd {
	userInput := m.ui.textarea.Value()
	if userInput == "" {
		return nil
	}

	m.messages = append(m.messages, message{mType: userMessage, content: userInput})
	m.ui.viewport.SetContent(m.renderConversation())
	m.ui.textarea.Reset()
	m.ui.showSpinner = true
	m.ui.textarea.Blur()

	// Reset the flag for the new conversation turn
	m.stream.streamingWasInterrupted = false

	return tea.Batch(m.ui.spinner.Tick, m.streamingCommand(userInput))
}

// selectModel handles model selection
func (m *model) selectModel() tea.Cmd {
	// Update the agent's model
	m.config.agent.Model = m.config.availableModels[m.ui.selectedModelIndex]
	m.ui.modelSelectionMode = false
	m.ui.textarea.Focus()

	// Save the selected model to preferences
	prefs := &config.UserPreferences{
		SelectedModel: m.config.agent.Model,
	}
	if err := config.SavePreferences(prefs); err != nil {
		// Log error but don't fail the operation
		m.messages = append(m.messages, message{
			mType:   agentMessage,
			content: fmt.Sprintf("Model switched to: %s (failed to save preference: %v)", m.config.agent.Model, err),
			isError: true,
		})
	} else {
		// Add a message to show model change
		m.messages = append(m.messages, message{
			mType:   agentMessage,
			content: fmt.Sprintf("Model switched to: %s", m.config.agent.Model),
		})
	}

	m.ui.viewport.SetContent(m.renderConversation())
	m.ui.viewport.GotoBottom()
	return nil
}

// handleStreamStart handles the start of streaming
func (m *model) handleStreamStart(msg streamStartMsg) tea.Cmd {
	// Start the real-time streaming process
	go func() {
		// Create context for this streaming session
		ctx := context.Background()
		// Call the agent's ProcessMessage for streaming with tool callback
		response, err := m.config.agent.ProcessMessage(ctx, msg.userInput,
			// Text callback for streaming chunks
			func(chunk string) error {
				select {
				case m.stream.streamChunkChan <- streamChunkMsg(chunk):
				default:
					// Channel full, skip chunk to avoid blocking
				}
				return nil
			},
			// Tool callback for immediate tool message display
			func(toolMsg agent.Message) error {
				select {
				case m.stream.toolMessageChan <- toolMessageMsg(toolMsg):
				default:
					// Channel full, skip to avoid blocking
				}
				return nil
			},
			// Thought callback for immediate thought message display
			func(thoughtMsg agent.Message) error {
				select {
				case m.stream.thoughtMessageChan <- thoughtMessageMsg(thoughtMsg):
				default:
					// Channel full, skip to avoid blocking
				}
				return nil
			},
			// Tool confirmation callback
			func(toolName string, args map[string]interface{}) (bool, error) {
				// If confirmation is not required, auto-approve
				if !m.config.requireToolConfirmation {
					return true, nil
				}

				// Create a response channel
				responseChan := make(chan bool, 1)

				// Send confirmation request to the UI
				m.stream.toolConfirmationChan <- toolConfirmationRequestMsg{
					toolName: toolName,
					args:     args,
					response: responseChan,
				}

				// Wait for user response
				select {
				case confirmed := <-responseChan:
					return confirmed, nil
				case <-ctx.Done():
					return false, ctx.Err()
				}
			},
			m.config.enableThinkingMode) // Pass thinking mode preference

		if err != nil {
			m.stream.streamCompleteChan <- streamCompleteMsg{
				finalMessages: []agent.Message{
					{Type: agent.AgentMessage, Content: fmt.Sprintf("Error: %v", err), IsError: true},
				},
			}
			return
		}

		// Send completion with all messages
		m.stream.streamCompleteChan <- streamCompleteMsg{finalMessages: response}
	}()

	// Start listening for chunks, tool messages, and completion
	return tea.Batch(
		waitForStreamChunk(m.stream.streamChunkChan),
		waitForToolMessage(m.stream.toolMessageChan),
		waitForThoughtMessage(m.stream.thoughtMessageChan),
		waitForStreamComplete(m.stream.streamCompleteChan),
		waitForToolConfirmation(m.stream.toolConfirmationChan),
	)
}

// handleToolMessage handles incoming tool messages
func (m *model) handleToolMessage(msg toolMessageMsg) tea.Cmd {
	// Handle tool message immediately
	newToolMsg := message{
		mType:       toolMessage,
		content:     msg.Content,
		isCollapsed: true,
		isError:     msg.IsError,
	}

	// Mark that streaming was interrupted only if we have an active streaming message
	if m.stream.streamingMsg != nil && m.stream.streamingMsg.content != "" {
		m.stream.streamingWasInterrupted = true
	}

	// If streaming has started, insert the tool message before the streaming message
	if m.stream.streamingMsgIndex != -1 {
		// Insert at the correct position
		m.messages = append(m.messages[:m.stream.streamingMsgIndex], append([]message{newToolMsg}, m.messages[m.stream.streamingMsgIndex:]...)...)
		// Update the index of the streaming message
		m.stream.streamingMsgIndex++
	} else {
		// Otherwise, just append
		m.messages = append(m.messages, newToolMsg)
	}

	m.ui.viewport.SetContent(m.renderConversation())
	m.ui.viewport.GotoBottom()

	// Continue listening for tool messages
	return waitForToolMessage(m.stream.toolMessageChan)
}

// handleThoughtMessage handles incoming thought messages
func (m *model) handleThoughtMessage(msg thoughtMessageMsg) tea.Cmd {
	// Handle thought message immediately
	newThoughtMsg := message{
		mType:       thoughtMessage,
		content:     msg.Content,
		isCollapsed: true,
		isError:     msg.IsError,
	}

	// Mark that streaming was interrupted only if we have an active streaming message
	if m.stream.streamingMsg != nil && m.stream.streamingMsg.content != "" {
		m.stream.streamingWasInterrupted = true
	}

	// If streaming has started, insert the thought message before the streaming message
	if m.stream.streamingMsgIndex != -1 {
		// Insert at the correct position
		m.messages = append(m.messages[:m.stream.streamingMsgIndex], append([]message{newThoughtMsg}, m.messages[m.stream.streamingMsgIndex:]...)...)
		// Update the index of the streaming message
		m.stream.streamingMsgIndex++
	} else {
		// Otherwise, just append
		m.messages = append(m.messages, newThoughtMsg)
	}

	m.ui.viewport.SetContent(m.renderConversation())
	m.ui.viewport.GotoBottom()

	// Continue listening for thought messages
	return waitForThoughtMessage(m.stream.thoughtMessageChan)
}

// handleStreamChunk handles incoming stream chunks
func (m *model) handleStreamChunk(msg streamChunkMsg) tea.Cmd {
	// Create streaming message if it doesn't exist yet
	if m.stream.streamingMsg == nil {
		m.stream.streamingMsg = &message{mType: agentMessage, content: "", isStreaming: true}
		m.messages = append(m.messages, *m.stream.streamingMsg)
		m.stream.streamingMsgIndex = len(m.messages) - 1 // Store the actual index
	}

	if m.stream.streamingMsg != nil {
		// If streaming was interrupted and is now resuming, add a newline
		if m.stream.streamingWasInterrupted {
			m.stream.streamingMsg.content += "\n\n"
			m.stream.streamingWasInterrupted = false
		}

		m.stream.streamingMsg.content += string(msg)
		// Update the streaming message at its tracked index
		if m.stream.streamingMsgIndex < len(m.messages) {
			m.messages[m.stream.streamingMsgIndex] = *m.stream.streamingMsg
		}
		m.ui.viewport.SetContent(m.renderConversation())
		m.ui.viewport.GotoBottom()
	}
	// Continue listening for more chunks
	return waitForStreamChunk(m.stream.streamChunkChan)
}

// handleStreamComplete handles stream completion
func (m *model) handleStreamComplete(msg streamCompleteMsg) tea.Cmd {
	// Handle streaming completion
	m.ui.showSpinner = false
	m.ui.textarea.Focus()

	// Finalize the streaming message
	if m.stream.streamingMsg != nil {
		m.stream.streamingMsg.isStreaming = false
		m.stream.streamingMsg = nil
		m.stream.streamingMsgIndex = -1 // Reset the index
	}

	// Reset the flag
	m.stream.streamingWasInterrupted = false

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
		case agent.AgentMessage:
			// Only process agent messages if they are errors
			// Normal agent messages were already displayed via streaming
			if agentMsg.IsError {
				newMsg := message{
					mType:   agentMessage,
					content: agentMsg.Content,
					isError: agentMsg.IsError,
				}
				m.messages = append(m.messages, newMsg)
			}
		}
	}

	m.ui.viewport.SetContent(m.renderConversation())
	m.ui.viewport.GotoBottom()
	return nil
}

// handleToolConfirmationRequest handles tool confirmation requests
func (m *model) handleToolConfirmationRequest(msg toolConfirmationRequestMsg) tea.Cmd {
	// Handle tool confirmation request
	m.ui.toolConfirmationMode = true
	m.ui.toolConfirmationName = msg.toolName
	m.ui.toolConfirmationArgs = msg.args
	m.stream.confirmationResponseChan = msg.response
	m.ui.textarea.Blur()
	// Continue listening for more confirmation requests
	return waitForToolConfirmation(m.stream.toolConfirmationChan)
}

func (m *model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	// Tool confirmation overlay takes priority
	if m.ui.toolConfirmationMode {
		return m.renderToolConfirmation(m.renderMainView())
	}

	// Model selector overlay
	if m.ui.modelSelectionMode {
		return m.renderModelSelector(m.renderMainView())
	}

	return m.renderMainView()
}

func (m *model) renderMainView() string {
	var taView string
	if m.ui.showSpinner {
		// Create a centered spinner with modern styling
		spinner := m.ui.spinner.View() + " Processing your request..."
		taView = textInputContainerStyle.
			Width(m.ui.width - 4).
			Render(
				lipgloss.NewStyle().
					Width(m.ui.width-12). // Account for container padding
					Align(lipgloss.Center, lipgloss.Center).
					Render(spinner),
			)
	} else {
		// Style the textarea with the modern container
		taView = textInputContainerStyle.
			Width(m.ui.width - 4).
			Render(m.ui.textarea.View())
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.ui.viewport.View(),
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

// waitForThoughtMessage creates a command that waits for the next thought message
func waitForThoughtMessage(ch <-chan thoughtMessageMsg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

// waitForToolConfirmation creates a command that waits for tool confirmation requests
func waitForToolConfirmation(ch <-chan toolConfirmationRequestMsg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

// A message for streaming content chunks
type streamChunkMsg string

// A message for tool messages during streaming
type toolMessageMsg agent.Message

// A message for thought messages during streaming
type thoughtMessageMsg agent.Message

// A message for streaming completion
type streamCompleteMsg struct {
	finalMessages []agent.Message
}

// A message for tool confirmation request
type toolConfirmationRequestMsg struct {
	toolName string
	args     map[string]interface{}
	response chan bool
}

// New message types for real-time streaming
type streamStartMsg struct {
	userInput string
}

// renderModelSelector renders the model selection overlay with modern styling
func (m *model) renderModelSelector(background string) string {
	// Create title with icon
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		MarginBottom(2).
		Render("🔮 Select AI Model")

	// Build the model list
	var modelItems []string
	for i, modelName := range m.config.availableModels {
		var itemStyle lipgloss.Style
		var prefix string

		// Check if this is the current model
		if modelName == m.config.agent.Model {
			prefix = "• "
		} else {
			prefix = "  "
		}

		// Apply selection styling
		if i == m.ui.selectedModelIndex {
			itemStyle = modelItemSelectedStyle
		} else {
			itemStyle = modelItemStyle
		}

		// Format model name with capabilities hint
		modelDisplay := modelName
		if strings.Contains(modelName, "pro") {
			modelDisplay += " (Advanced)"
		} else if strings.Contains(modelName, "flash-lite") {
			modelDisplay += " (Fast & Light)"
		} else if strings.Contains(modelName, "flash") {
			modelDisplay += " (Fast)"
		}

		modelItems = append(modelItems, itemStyle.Render(prefix+modelDisplay))
	}

	modelList := lipgloss.JoinVertical(lipgloss.Left, modelItems...)

	// Add navigation help
	navHelp := lipgloss.NewStyle().
		Foreground(textMuted).
		MarginTop(2).
		Align(lipgloss.Center).
		Render("↑/↓ Navigate • Enter Select • Esc Cancel")

	// Combine all elements
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		modelList,
		navHelp,
	)

	// Apply the modern selector styling
	selectorBox := modelSelectorStyle.
		Width(50). // Fixed width for consistency
		Render(content)

	// Position the selector in the center
	return lipgloss.Place(
		m.ui.width, m.ui.height,
		lipgloss.Center, lipgloss.Center,
		selectorBox,
	)
}

// renderToolConfirmation renders the tool confirmation overlay
func (m *model) renderToolConfirmation(background string) string {
	// Create the confirmation box with modern styling
	confirmStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(warningColor).
		Padding(2, 3).
		Background(bgMedium)

	// Title with warning icon
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(warningColor).
		Align(lipgloss.Center).
		MarginBottom(2)

	title := titleStyle.Render("⚠️  Tool Execution Request")

	// Tool name section
	toolNameStyle := lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true).
		MarginBottom(1)

	toolNameSection := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Foreground(textMuted).Render("Tool: "),
		toolNameStyle.Render(m.ui.toolConfirmationName),
	)

	// Arguments section with syntax highlighting
	argsHeaderStyle := lipgloss.NewStyle().
		Foreground(textMuted).
		MarginTop(1).
		MarginBottom(1)

	argsHeader := argsHeaderStyle.Render("Arguments:")

	// Format arguments with proper indentation and coloring
	argsJSON, _ := json.MarshalIndent(m.ui.toolConfirmationArgs, "", "  ")
	argsStyle := lipgloss.NewStyle().
		Foreground(secondaryColor).
		Background(bgDark).
		Padding(1).
		Border(lipgloss.NormalBorder()).
		BorderForeground(bgLighter)

	argsContent := argsStyle.Render(string(argsJSON))

	// Question section
	questionStyle := lipgloss.NewStyle().
		Foreground(textPrimary).
		Bold(true).
		MarginTop(2).
		MarginBottom(2).
		Align(lipgloss.Center)

	question := questionStyle.Render("Do you want to execute this tool?")

	// Action buttons visualization
	buttonStyle := lipgloss.NewStyle().
		Padding(0, 2).
		MarginRight(2)

	yesButton := buttonStyle.Copy().
		Background(accentColor).
		Foreground(bgDark).
		Bold(true).
		Render("Y - Yes")

	noButton := buttonStyle.Copy().
		Background(errorColor).
		Foreground(textPrimary).
		Bold(true).
		Render("N - No")

	escButton := buttonStyle.Copy().
		Background(bgLighter).
		Foreground(textPrimary).
		Render("Esc - Cancel")

	buttons := lipgloss.JoinHorizontal(
		lipgloss.Top,
		yesButton,
		noButton,
		escButton,
	)

	buttonsContainer := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(50). // Fixed width for centering
		Render(buttons)

	// Security note
	securityNote := lipgloss.NewStyle().
		Foreground(textMuted).
		Italic(true).
		MarginTop(2).
		Align(lipgloss.Center).
		Render("🔒 Tool execution requires your permission")

	// Combine all elements
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		toolNameSection,
		argsHeader,
		argsContent,
		question,
		buttonsContainer,
		securityNote,
	)

	// Apply confirmation box styling
	confirmBox := confirmStyle.
		Width(60). // Fixed width for consistency
		Render(content)

	// Create semi-transparent overlay effect
	return lipgloss.Place(
		m.ui.width, m.ui.height,
		lipgloss.Center, lipgloss.Center,
		confirmBox,
		lipgloss.WithWhitespaceBackground(bgDark),
	)
}
