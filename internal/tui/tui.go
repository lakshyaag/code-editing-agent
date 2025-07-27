package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent/internal/agent"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

type (
	sessionState int
	messageType  int
	tabMode      int
	message      struct {
		mType       messageType
		content     string
		isCollapsed bool
		isError     bool
		timestamp   time.Time
	}
	fileItem struct {
		name     string
		path     string
		isDir    bool
		size     int64
		modified time.Time
	}
)

const (
	userMessage messageType = iota
	agentMessage
	toolMessage
)

const (
	chatTab tabMode = iota
	filesTab
	settingsTab
	helpTab
)

// Animation states
const (
	animationDuration = time.Millisecond * 300
)

// A message to indicate that the agent has finished processing
type agentResponseMsg []agent.Message

// Progress update message
type progressMsg float64

// Animation tick message
type animTickMsg struct{}

// File list loaded message
type fileListMsg []fileItem

type model struct {
	// Core components
	viewport       viewport.Model
	textarea       textarea.Model
	spinner        spinner.Model
	progress       progress.Model
	fileList       list.Model
	fileTable      table.Model
	agent          *agent.Agent

	// State
	currentTab     tabMode
	err            error
	showSpinner    bool
	messages       []message
	width, height  int
	files          []fileItem
	currentDir     string

	// Animation state
	isAnimating   bool

	// Mouse zones
	zone *zone.Manager

	// Styles cache
	styles *Styles
	
	// Tool message collapse state
	toolMessageZones map[string]int
}

type Styles struct {
	// Tab styles
	TabActive   lipgloss.Style
	TabInactive lipgloss.Style
	TabBar      lipgloss.Style

	// Message styles
	UserMessage  lipgloss.Style
	AgentMessage lipgloss.Style
	ToolMessage  lipgloss.Style
	ErrorMessage lipgloss.Style

	// UI component styles
	StatusBar    lipgloss.Style
	Sidebar      lipgloss.Style
	MainContent  lipgloss.Style
	Border       lipgloss.Style
	Highlight    lipgloss.Style
	Muted        lipgloss.Style

	// Special effects
	Glow      lipgloss.Style
	Gradient  lipgloss.Style
	Shadow    lipgloss.Style
}

func newStyles() *Styles {
	// Color palette
	primaryColor := lipgloss.Color("86")      // Bright cyan
	secondaryColor := lipgloss.Color("213")   // Pink
	accentColor := lipgloss.Color("11")       // Yellow
	bgColor := lipgloss.Color("0")            // Black
	surfaceColor := lipgloss.Color("235")     // Dark gray
	textColor := lipgloss.Color("15")         // White
	mutedColor := lipgloss.Color("245")       // Light gray
	errorColor := lipgloss.Color("196")       // Red

	return &Styles{
		// Tab styles
		TabActive: lipgloss.NewStyle().
			Foreground(bgColor).
			Background(primaryColor).
			Padding(0, 2).
			Bold(true).
			MarginRight(1),

		TabInactive: lipgloss.NewStyle().
			Foreground(mutedColor).
			Background(surfaceColor).
			Padding(0, 2).
			MarginRight(1),

		TabBar: lipgloss.NewStyle().
			Background(surfaceColor).
			Padding(1, 0),

		// Message styles
		UserMessage: lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			MarginBottom(1),

		AgentMessage: lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true).
			MarginBottom(1),

		ToolMessage: lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true).
			MarginBottom(1),

		ErrorMessage: lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true).
			MarginBottom(1),

		// UI component styles
		StatusBar: lipgloss.NewStyle().
			Background(surfaceColor).
			Foreground(textColor).
			Padding(0, 1),

		Sidebar: lipgloss.NewStyle().
			Background(surfaceColor).
			Padding(1).
			Width(25),

		MainContent: lipgloss.NewStyle().
			Padding(1),

		Border: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor),

		Highlight: lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true),

		Muted: lipgloss.NewStyle().
			Foreground(mutedColor),

		// Special effects
		Glow: lipgloss.NewStyle().
			Foreground(primaryColor).
			Background(primaryColor).
			Faint(true),

		Gradient: lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Background(lipgloss.Color("5")),

		Shadow: lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")),
	}
}

func InitialModel(agent *agent.Agent) *model {
	// Initialize text area
	ta := textarea.New()
	ta.Placeholder = "✨ Ask me anything..."
	ta.Focus()
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.CharLimit = 2000

	// Initialize spinner with custom style
	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

	// Initialize viewport
	vp := viewport.New(80, 20)
	vp.SetContent("🤖 Welcome to the Enhanced AI Agent!\n\nI'm here to help you with your coding tasks. Use the tabs above to navigate between different modes.")

	// Initialize progress bar
	prog := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	// Initialize file list
	fileListItems := []list.Item{}
	fl := list.New(fileListItems, list.NewDefaultDelegate(), 0, 0)
	fl.Title = "📁 File Explorer"
	fl.SetShowStatusBar(false)
	fl.SetFilteringEnabled(true)

	// Initialize file table
	columns := []table.Column{
		{Title: "Name", Width: 30},
		{Title: "Size", Width: 10},
		{Title: "Modified", Width: 15},
	}
	ft := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(7),
	)
	tableStyle := table.DefaultStyles()
	tableStyle.Header = tableStyle.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	tableStyle.Selected = tableStyle.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	ft.SetStyles(tableStyle)

	// Initialize current directory
	cwd, _ := os.Getwd()

	return &model{
		textarea:      ta,
		viewport:      vp,
		spinner:       s,
		progress:      prog,
		fileList:      fl,
		fileTable:     ft,
		agent:         agent,
		currentTab:    chatTab,
		showSpinner:   false,
		messages:         []message{{mType: agentMessage, content: "🤖 Welcome to the Enhanced AI Agent!\n\nI'm here to help you with your coding tasks. Use the tabs above to navigate between different modes.", timestamp: time.Now()}},
		currentDir:       cwd,
		zone:             zone.New(),
		styles:           newStyles(),
		toolMessageZones: make(map[string]int),
	}
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
		tea.Tick(time.Millisecond*16, func(t time.Time) tea.Msg {
			return animTickMsg{}
		}),
		m.loadFileList(),
	)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	// Handle zone mouse events first (zone doesn't need update call)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()
		return m, nil

	case animTickMsg:
		// Simple animation tick - we'll keep the animation simple for now
		if m.isAnimating {
			// Continue animation for a short duration
			m.isAnimating = false // Disable complex animations for now
		}

	case tea.MouseMsg:
		// Handle tab clicks
		for i := 0; i < 4; i++ {
			tabID := fmt.Sprintf("tab-%d", i)
			if m.zone.Get(tabID).InBounds(msg) && msg.Type == tea.MouseLeft {
				m.currentTab = tabMode(i)
				m.startTabAnimation()
				m.updateFocus()
				return m, tea.Tick(time.Millisecond*16, func(t time.Time) tea.Msg {
					return animTickMsg{}
				})
			}
		}
		
		// Handle tool message clicks
		for zoneID, msgIndex := range m.toolMessageZones {
			if m.zone.Get(zoneID).InBounds(msg) && msg.Type == tea.MouseLeft {
				if msgIndex < len(m.messages) {
					m.messages[msgIndex].isCollapsed = !m.messages[msgIndex].isCollapsed
					m.viewport.SetContent(m.renderConversation())
					return m, nil
				}
			}
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "tab":
			m.nextTab()
			return m, tea.Tick(time.Millisecond*16, func(t time.Time) tea.Msg {
				return animTickMsg{}
			})
		case "shift+tab":
			m.prevTab()
			return m, tea.Tick(time.Millisecond*16, func(t time.Time) tea.Msg {
				return animTickMsg{}
			})
		case "f5":
			if m.currentTab == filesTab {
				return m, m.loadFileList()
			}
		case "enter":
			if m.currentTab == chatTab && m.textarea.Focused() {
				userInput := m.textarea.Value()
				if userInput == "" {
					return m, nil
				}

				m.addMessage(userMessage, userInput, false)
				m.textarea.Reset()
				m.showSpinner = true
				m.textarea.Blur()

				return m, tea.Batch(
					m.spinner.Tick,
					m.progress.SetPercent(0.1), // Start progress
					func() tea.Msg {
						response, err := m.agent.ProcessMessage(context.Background(), userInput)
						if err != nil {
							return agentResponseMsg{
								{Type: agent.AgentMessage, Content: fmt.Sprintf("❌ Error: %v", err), IsError: true},
							}
						}
						return agentResponseMsg(response)
					},
				)
			}
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
			m.addMessage(mType, msg.Content, msg.IsError)
		}
		m.viewport.GotoBottom()
		// Complete the progress
		return m, m.progress.SetPercent(1.0)

	case fileListMsg:
		m.files = msg
		m.updateFileComponents()
		return m, nil

	case progressMsg:
		return m, m.progress.SetPercent(float64(msg))

	case error:
		m.err = msg
		return m, nil
	}

	// Update components based on current tab
	switch m.currentTab {
	case chatTab:
		if m.textarea.Focused() {
			m.textarea, cmd = m.textarea.Update(msg)
			cmds = append(cmds, cmd)
		}
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)

	case filesTab:
		m.fileList, cmd = m.fileList.Update(msg)
		cmds = append(cmds, cmd)
		m.fileTable, cmd = m.fileTable.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Always update spinner
	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *model) View() string {
	if m.err != nil {
		return fmt.Sprintf("❌ Error: %v\n\nPress Ctrl+C to exit", m.err)
	}

	// Render with zone management
	return m.zone.Scan(lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderHeader(),
		m.renderTabs(),
		m.renderContent(),
		m.renderStatusBar(),
	))
}

func (m *model) renderHeader() string {
	title := m.styles.Highlight.Render("🚀 Enhanced AI Agent")
	subtitle := m.styles.Muted.Render("Powered by Bubble Tea & Lipgloss")
	
	header := lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		subtitle,
	)

	return lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Padding(1, 0).
		Background(lipgloss.Color("235")).
		Render(header)
}

func (m *model) renderTabs() string {
	tabs := []string{"💬 Chat", "📁 Files", "⚙️  Settings", "❓ Help"}
	var renderedTabs []string

	for i, tab := range tabs {
		var style lipgloss.Style
		if tabMode(i) == m.currentTab {
			style = m.styles.TabActive
		} else {
			style = m.styles.TabInactive
		}
		
		// Add mouse zone
		tabID := fmt.Sprintf("tab-%d", i)
		clickableTab := m.zone.Mark(tabID, style.Render(tab))
		
		// Tab will be clickable through zone management
		
		renderedTabs = append(renderedTabs, clickableTab)
	}

	return m.styles.TabBar.Width(m.width).Render(
		lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...),
	)
}

func (m *model) renderContent() string {
	contentHeight := m.height - 8 // Account for header, tabs, and status bar
	
	switch m.currentTab {
	case chatTab:
		return m.renderChatTab(contentHeight)
	case filesTab:
		return m.renderFilesTab(contentHeight)
	case settingsTab:
		return m.renderSettingsTab(contentHeight)
	case helpTab:
		return m.renderHelpTab(contentHeight)
	default:
		return m.renderChatTab(contentHeight)
	}
}

func (m *model) renderChatTab(height int) string {
	// Adjust viewport height
	m.viewport.Height = height - m.textarea.Height() - 2
	m.viewport.Width = m.width

	var taView string
	if m.showSpinner {
		spinner := m.spinner.View() + " " + m.styles.Muted.Render("Agent is thinking...")
		progress := m.progress.View()
		loadingContent := lipgloss.JoinVertical(
			lipgloss.Center,
			spinner,
			progress,
		)
		taView = lipgloss.NewStyle().
			Width(m.width).
			Height(m.textarea.Height()).
			Align(lipgloss.Center, lipgloss.Center).
			Render(loadingContent)
	} else {
		taView = m.textarea.View()
	}

	// Update conversation content
	m.viewport.SetContent(m.renderConversation())

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.viewport.View(),
		taView,
	)
}

func (m *model) renderFilesTab(height int) string {
	// Two-column layout: file list and file table
	m.fileList.SetSize(m.width/2-2, height-2)
	
	leftColumn := m.styles.Border.
		Height(height-2).
		Width(m.width/2-1).
		Render(m.fileList.View())
	
	rightColumn := m.styles.Border.
		Height(height-2).
		Width(m.width/2-1).
		Render(m.fileTable.View())

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftColumn,
		rightColumn,
	)
}

func (m *model) renderSettingsTab(height int) string {
	settings := []string{
		"🎨 Theme: Dark Mode",
		"🤖 Model: " + m.agent.Model,
		"📂 Current Directory: " + m.currentDir,
		"⌨️  Keybindings:",
		"  • Tab/Shift+Tab: Switch tabs",
		"  • F5: Refresh file list",
		"  • Ctrl+C/Esc: Exit",
		"",
		"💡 Tip: Use the file browser to explore your project!",
	}

	content := strings.Join(settings, "\n")
	
	return m.styles.Border.
		Height(height-2).
		Width(m.width-2).
		Render(
			m.styles.MainContent.Render(content),
		)
}

func (m *model) renderHelpTab(height int) string {
	help := []string{
		"🚀 Enhanced AI Agent Help",
		"",
		"📋 Available Features:",
		"",
		"💬 Chat Tab:",
		"  • Interactive conversation with AI agent",
		"  • Tool execution and file operations",
		"  • Syntax highlighting and formatting",
		"",
		"📁 Files Tab:",
		"  • Browse project files and directories",
		"  • File size and modification info",
		"  • Quick navigation and filtering",
		"",
		"⚙️  Settings Tab:",
		"  • View current configuration",
		"  • Keyboard shortcuts reference",
		"  • Theme and model information",
		"",
		"🎨 UI Features:",
		"  • Smooth animations and transitions",
		"  • Mouse support with clickable zones",
		"  • Rich styling with Lipgloss",
		"  • Progress indicators and spinners",
		"",
		"⌨️  Keyboard Shortcuts:",
		"  • Tab/Shift+Tab: Navigate between tabs",
		"  • Enter: Send message (in chat)",
		"  • F5: Refresh file list",
		"  • Ctrl+C/Esc: Exit application",
	}

	content := strings.Join(help, "\n")
	
	return m.styles.Border.
		Height(height-2).
		Width(m.width-2).
		Render(
			m.styles.MainContent.Render(content),
		)
}

func (m *model) renderConversation() string {
	var lines []string
	m.toolMessageZones = make(map[string]int) // Reset zones

	for i, msg := range m.messages {
		var renderedBlock string
		timestamp := m.styles.Muted.Render(msg.timestamp.Format("15:04:05"))
		
		switch msg.mType {
		case userMessage:
			prefix := m.styles.UserMessage.Render("👤 You:")
			content := wrapText(msg.content, m.viewport.Width-10)
			renderedBlock = lipgloss.JoinVertical(
				lipgloss.Left,
				lipgloss.JoinHorizontal(lipgloss.Top, prefix, " ", timestamp),
				content,
				"",
			)
		case agentMessage:
			prefix := m.styles.AgentMessage.Render("🤖 Agent:")
			content := wrapText(msg.content, m.viewport.Width-10)
			if msg.isError {
				content = m.styles.ErrorMessage.Render(content)
			}
			renderedBlock = lipgloss.JoinVertical(
				lipgloss.Left,
				lipgloss.JoinHorizontal(lipgloss.Top, prefix, " ", timestamp),
				content,
				"",
			)
		case toolMessage:
			style := m.styles.ToolMessage
			if msg.isError {
				style = m.styles.ErrorMessage
			}

			zoneID := fmt.Sprintf("tool-%d", i)
			m.toolMessageZones[zoneID] = i

			if msg.isCollapsed {
				header := style.Render("🔧 [+] Tool execution")
				clickableHeader := m.zone.Mark(zoneID, header)
				renderedBlock = lipgloss.JoinHorizontal(lipgloss.Top, clickableHeader, " ", timestamp)
			} else {
				header := style.Render("🔧 [-] Tool execution")
				clickableHeader := m.zone.Mark(zoneID, header)
				content := wrapText(msg.content, m.viewport.Width-4)
				renderedBlock = lipgloss.JoinVertical(
					lipgloss.Left,
					lipgloss.JoinHorizontal(lipgloss.Top, clickableHeader, " ", timestamp),
					style.Render(content),
					"",
				)
			}
		}
		lines = append(lines, renderedBlock)
	}
	return strings.Join(lines, "\n")
}

func (m *model) renderStatusBar() string {
	// Left side status
	leftStatus := []string{
		fmt.Sprintf("Tab: %s", m.getTabName()),
		fmt.Sprintf("Messages: %d", len(m.messages)),
	}

	// Right side status
	rightStatus := []string{
		fmt.Sprintf("Size: %dx%d", m.width, m.height),
		fmt.Sprintf("Dir: %s", filepath.Base(m.currentDir)),
	}

	left := strings.Join(leftStatus, " • ")
	right := strings.Join(rightStatus, " • ")

	// Calculate spacing
	totalWidth := lipgloss.Width(left) + lipgloss.Width(right)
	spacing := m.width - totalWidth

	status := lipgloss.JoinHorizontal(
		lipgloss.Top,
		left,
		strings.Repeat(" ", max(0, spacing)),
		right,
	)

	return m.styles.StatusBar.Width(m.width).Render(status)
}

// Helper functions

func (m *model) nextTab() {
	m.currentTab = tabMode((int(m.currentTab) + 1) % 4)
	m.startTabAnimation()
	m.updateFocus()
}

func (m *model) prevTab() {
	m.currentTab = tabMode((int(m.currentTab) + 3) % 4)
	m.startTabAnimation()
	m.updateFocus()
}

func (m *model) startTabAnimation() {
	// Simple animation trigger
	m.isAnimating = true
}

func (m *model) updateFocus() {
	switch m.currentTab {
	case chatTab:
		m.textarea.Focus()
	default:
		m.textarea.Blur()
	}
}

func (m *model) getTabName() string {
	names := []string{"Chat", "Files", "Settings", "Help"}
	return names[m.currentTab]
}

func (m *model) addMessage(mType messageType, content string, isError bool) {
	msg := message{
		mType:       mType,
		content:     content,
		isError:     isError,
		timestamp:   time.Now(),
		isCollapsed: mType == toolMessage, // Collapse tool messages by default
	}
	m.messages = append(m.messages, msg)
	// Simple animation trigger
	m.isAnimating = true
}

func (m *model) updateLayout() {
	// Update component sizes based on window size
	m.textarea.SetWidth(m.width)
	m.viewport.Width = m.width
}

func (m *model) loadFileList() tea.Cmd {
	return func() tea.Msg {
		var files []fileItem
		
		err := filepath.Walk(m.currentDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}
			
			// Skip hidden files and directories
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			
			// Don't include the root directory itself
			if path == m.currentDir {
				return nil
			}
			
			relPath, _ := filepath.Rel(m.currentDir, path)
			if strings.Contains(relPath, string(filepath.Separator)) {
				return nil // Only immediate children
			}
			
			files = append(files, fileItem{
				name:     info.Name(),
				path:     path,
				isDir:    info.IsDir(),
				size:     info.Size(),
				modified: info.ModTime(),
			})
			
			return nil
		})
		
		if err != nil {
			return fileListMsg{}
		}
		
		return fileListMsg(files)
	}
}

func (m *model) updateFileComponents() {
	// Update file list
	var items []list.Item
	for _, file := range m.files {
		items = append(items, file)
	}
	m.fileList.SetItems(items)
	
	// Update file table
	var rows []table.Row
	for _, file := range m.files {
		icon := "📄"
		if file.isDir {
			icon = "📁"
		}
		
		size := formatSize(file.size)
		if file.isDir {
			size = "-"
		}
		
		rows = append(rows, table.Row{
			icon + " " + file.name,
			size,
			file.modified.Format("Jan 02 15:04"),
		})
	}
	m.fileTable.SetRows(rows)
}

// Implement list.Item interface for fileItem
func (f fileItem) FilterValue() string { return f.name }

func wrapText(text string, width int) string {
	return lipgloss.NewStyle().Width(width).Render(text)
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func Start(agent *agent.Agent) {
	// Enable mouse support and alt screen
	p := tea.NewProgram(
		InitialModel(agent),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}
