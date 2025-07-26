package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/rivo/tview"
)

// ChatMessage represents a single chat message
type ChatMessage struct {
	Role      string    // "user", "assistant", "system", "tool"
	Content   string
	Timestamp time.Time
	ToolCall  string // For tool execution messages
}

// ChatUI manages the chat interface
type ChatUI struct {
	*tview.TextView
	messages       []ChatMessage
	showTimestamps bool
}

// NewChatUI creates a new chat UI component
func NewChatUI() *ChatUI {
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true).
		SetScrollable(true)

	textView.
		SetBorder(true).
		SetTitle(" Chat ").
		SetTitleAlign(tview.AlignLeft)

	chat := &ChatUI{
		TextView:       textView,
		messages:       make([]ChatMessage, 0),
		showTimestamps: true,
	}

	// Add welcome message
	chat.AddSystemMessage("Welcome! Start chatting with your AI assistant.")

	return chat
}

// AddUserMessage adds a user message to the chat
func (c *ChatUI) AddUserMessage(content string) {
	message := ChatMessage{
		Role:      "user",
		Content:   content,
		Timestamp: time.Now(),
	}
	c.messages = append(c.messages, message)
	c.updateDisplay()
}

// AddAssistantMessage adds an assistant message to the chat
func (c *ChatUI) AddAssistantMessage(content string) {
	message := ChatMessage{
		Role:      "assistant",
		Content:   content,
		Timestamp: time.Now(),
	}
	c.messages = append(c.messages, message)
	c.updateDisplay()
}

// AddToolMessage adds a tool execution message to the chat
func (c *ChatUI) AddToolMessage(toolName string, args string, result string) {
	message := ChatMessage{
		Role:      "tool",
		Content:   result,
		Timestamp: time.Now(),
		ToolCall:  fmt.Sprintf("%s(%s)", toolName, args),
	}
	c.messages = append(c.messages, message)
	c.updateDisplay()
}

// AddSystemMessage adds a system message to the chat
func (c *ChatUI) AddSystemMessage(content string) {
	message := ChatMessage{
		Role:      "system",
		Content:   content,
		Timestamp: time.Now(),
	}
	c.messages = append(c.messages, message)
	c.updateDisplay()
}

// SetShowTimestamps controls whether timestamps are displayed
func (c *ChatUI) SetShowTimestamps(show bool) {
	c.showTimestamps = show
	c.updateDisplay()
}

// updateDisplay refreshes the chat display with all messages
func (c *ChatUI) updateDisplay() {
	c.Clear()
	
	for i, msg := range c.messages {
		c.writeMessage(msg, i == len(c.messages)-1)
	}
	
	// Auto-scroll to bottom
	c.ScrollToEnd()
}

// writeMessage writes a single message to the display
func (c *ChatUI) writeMessage(msg ChatMessage, isLast bool) {
	var prefix, color string
	
	switch msg.Role {
	case "user":
		prefix = "You"
		color = "[#87CEEB]" // Sky blue
	case "assistant":
		prefix = "Assistant"
		color = "[#98FB98]" // Pale green
	case "system":
		prefix = "System"
		color = "[#FFB6C1]" // Light pink
	case "tool":
		prefix = "Tool"
		color = "[#DDA0DD]" // Plum
	default:
		prefix = "Unknown"
		color = "[white]"
	}
	
	// Format timestamp
	timestamp := ""
	if c.showTimestamps {
		timestamp = fmt.Sprintf(" [#666666](%s)[-]", msg.Timestamp.Format("15:04:05"))
	}
	
	// Write message header
	if msg.Role == "tool" && msg.ToolCall != "" {
		fmt.Fprintf(c, "%s%s%s: %s\n", color, prefix, timestamp, msg.ToolCall)
	} else {
		fmt.Fprintf(c, "%s%s%s:[-]\n", color, prefix, timestamp)
	}
	
	// Write message content with proper wrapping
	content := strings.TrimSpace(msg.Content)
	if content != "" {
		// Split long lines and indent content slightly
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			fmt.Fprintf(c, "  %s\n", line)
		}
	}
	
	// Add spacing between messages (except for the last one)
	if !isLast {
		fmt.Fprint(c, "\n")
	}
}

// Clear clears all messages from the chat
func (c *ChatUI) Clear() {
	c.TextView.Clear()
	c.messages = make([]ChatMessage, 0)
	c.AddSystemMessage("Chat cleared. Start a new conversation!")
}

// GetMessageCount returns the number of messages in the chat
func (c *ChatUI) GetMessageCount() int {
	return len(c.messages)
}

// GetMessages returns a copy of all messages
func (c *ChatUI) GetMessages() []ChatMessage {
	messages := make([]ChatMessage, len(c.messages))
	copy(messages, c.messages)
	return messages
}