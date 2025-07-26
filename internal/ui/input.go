package ui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// InputField manages the chat input interface
type InputField struct {
	*tview.TextArea
	onSend     func(string)
	onEscape   func()
	multiline  bool
	sendButton *tview.Button
}

// NewInputField creates a new input field component
func NewInputField(onSend func(string), onEscape func()) *InputField {
	textArea := tview.NewTextArea().
		SetPlaceholder("Type your message here... (Ctrl+Enter to send, Esc for options)").
		SetWrap(true)

	textArea.
		SetBorder(true).
		SetTitle(" Message Input ").
		SetTitleAlign(tview.AlignLeft)

	input := &InputField{
		TextArea:  textArea,
		onSend:    onSend,
		onEscape:  onEscape,
		multiline: false,
	}

	// Set up key handlers
	textArea.SetInputCapture(input.handleInput)

	return input
}

// handleInput handles keyboard input for the input field
func (i *InputField) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch {
	case event.Key() == tcell.KeyCtrlQ:
		// Ctrl+Q to quit
		return tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
		
	case event.Key() == tcell.KeyCtrlM || (event.Key() == tcell.KeyEnter && event.Modifiers()&tcell.ModCtrl != 0):
		// Ctrl+Enter to send message
		text := strings.TrimSpace(i.GetText())
		if text != "" && i.onSend != nil {
			i.onSend(text)
			i.SetText("", true)
		}
		return nil

	case event.Key() == tcell.KeyEnter && !i.multiline:
		// Regular Enter sends message in single-line mode
		text := strings.TrimSpace(i.GetText())
		if text != "" && i.onSend != nil {
			i.onSend(text)
			i.SetText("", true)
		}
		return nil

	case event.Key() == tcell.KeyEscape:
		// Escape key for options/commands
		if i.onEscape != nil {
			i.onEscape()
		}
		return nil

	case event.Key() == tcell.KeyCtrlL:
		// Ctrl+L to toggle multiline mode
		i.toggleMultilineMode()
		return nil
	}

	return event
}

// toggleMultilineMode toggles between single-line and multi-line input modes
func (i *InputField) toggleMultilineMode() {
	i.multiline = !i.multiline
	
	if i.multiline {
		i.SetTitle(" Message Input (Multi-line mode - Ctrl+Enter to send) ")
		i.SetPlaceholder("Type your message here... (Ctrl+Enter to send, Ctrl+L for single-line)")
	} else {
		i.SetTitle(" Message Input (Enter to send) ")
		i.SetPlaceholder("Type your message here... (Enter to send, Ctrl+L for multi-line)")
	}
}

// Focus sets focus to the input field
func (i *InputField) Focus(delegate func(p tview.Primitive)) {
	i.TextArea.Focus(delegate)
}

// SetMultilineMode sets the multiline mode
func (i *InputField) SetMultilineMode(multiline bool) {
	i.multiline = multiline
	i.toggleMultilineMode()
	// Call twice to get the right state since toggle flips it
	if i.multiline != multiline {
		i.toggleMultilineMode()
	}
}

// IsMultilineMode returns whether multiline mode is enabled
func (i *InputField) IsMultilineMode() bool {
	return i.multiline
}

// Clear clears the input field
func (i *InputField) Clear() {
	i.SetText("", true)
}

// GetInputText returns the current input text
func (i *InputField) GetInputText() string {
	return i.GetText()
}

// SetInputText sets the input text
func (i *InputField) SetInputText(text string) {
	i.SetText(text, true)
}