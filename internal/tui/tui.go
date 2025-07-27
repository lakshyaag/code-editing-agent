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
	}
)

const (
	userMessage messageType = iota
	agentMessage
	toolMessage
)

// A message to indicate that the agent has finished processing
type agentResponseMsg []agent.Message

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
		textarea:       ta,
		viewport:       vp,
		spinner:        s,
		agent:          agent,
		showSpinner:    false,
		messages:       []message{{mType: agentMessage, content: "Welcome to the AI Agent!"}},
		showStatusBar:  true,
		clickableLines: make(map[int]int),
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

			return m, tea.Batch(sCmd, func() tea.Msg {
				// Call the agent's ProcessMessage function
				response, err := m.agent.ProcessMessage(context.Background(), userInput)
				if err != nil {
					return agentResponseMsg{
						{Type: agent.AgentMessage, Content: fmt.Sprintf("Error: %v", err), IsError: true},
					}
				}
				return agentResponseMsg(response)
			})
		}
	case agentResponseMsg:
		m.showSpinner = false
		m.textarea.Focus()
		for _, msg := range msg {
			var mType messageType
			switch msg.Type {
			case agent.UserMessage:
				mType = userMessage
			case agent.AgentMessage:
				mType = agentMessage
			case agent.ToolMessage:
				mType = toolMessage
			}
			m.messages = append(m.messages, message{
				mType:       mType,
				content:     msg.Content,
				isCollapsed: mType == toolMessage,
				isError:     msg.IsError,
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
			wrappedContent := wrapText(msg.content, m.viewport.Width-lipgloss.Width(prefix))
			renderedBlock = lipgloss.JoinHorizontal(lipgloss.Top, agentStyle.Render(prefix), wrappedContent)
		case toolMessage:
			style := toolStyle
			if msg.isError {
				style = errorToolStyle
			}

			if msg.isCollapsed {
				header := style.Render("[+] Tool call")
				m.clickableLines[currentLine] = i
				renderedBlock = header
			} else {
				header := style.Render("[-] Tool call")
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

	status := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Padding(0, 1).Render(modelInfo),
		lipgloss.NewStyle().Padding(0, 1).Render(cwdInfo),
	)

	return lipgloss.NewStyle().
		Width(m.width).
		Background(lipgloss.Color("235")).
		Foreground(lipgloss.Color("250")).
		Render(status)
}

func Start(agent *agent.Agent) {
	p := tea.NewProgram(InitialModel(agent), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}

var (
	// Lipgloss styles
	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
)
