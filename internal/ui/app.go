package ui

import (
	"agent/internal/config"
	"agent/internal/models"
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// App represents the main TUI application
type App struct {
	app           *tview.Application
	pages         *tview.Pages
	chatUI        *ChatUI
	inputField    *InputField
	statusBar     *tview.TextView
	modelSelector *ModelSelector
	
	// Configuration and state
	config      *config.Config
	preferences *config.Preferences
	currentModel string
	
	// Callbacks
	onSendMessage   func(string)
	onModelChanged  func(string)
	onQuit          func()
}

// NewApp creates a new TUI application
func NewApp(cfg *config.Config, prefs *config.Preferences) *App {
	app := tview.NewApplication()
	pages := tview.NewPages()

	// Initialize components
	chatUI := NewChatUI()
	statusBar := createStatusBar(cfg.Model)
	
	uiApp := &App{
		app:         app,
		pages:       pages,
		chatUI:      chatUI,
		statusBar:   statusBar,
		config:      cfg,
		preferences: prefs,
		currentModel: prefs.LastSelectedModel,
	}

	// Create input field with callbacks
	inputField := NewInputField(
		func(message string) {
			if uiApp.onSendMessage != nil {
				uiApp.onSendMessage(message)
			}
		},
		uiApp.showOptionsMenu,
	)
	uiApp.inputField = inputField

	// Create model selector
	modelSelector := NewModelSelector(
		uiApp.currentModel,
		uiApp.handleModelSelection,
	)
	uiApp.modelSelector = modelSelector

	// Set up the main layout
	uiApp.setupLayout()

	// Set up global key handlers
	app.SetInputCapture(uiApp.handleGlobalKeys)

	return uiApp
}

// setupLayout creates the main application layout
func (a *App) setupLayout() {
	// Create main layout
	mainFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.chatUI, 0, 1, false).      // Chat takes most space
		AddItem(a.inputField, 4, 0, true).   // Input field at bottom
		AddItem(a.statusBar, 1, 0, false)    // Status bar at very bottom

	// Add to pages
	a.pages.AddPage("main", mainFlex, true, true)
	a.app.SetRoot(a.pages, true)
}

// createStatusBar creates the status bar at the bottom
func createStatusBar(modelName string) *tview.TextView {
	statusBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	
	status := fmt.Sprintf(
		"[yellow]Model:[-] %s | [yellow]Commands:[-] Esc (menu), Ctrl+Q (quit), Ctrl+L (toggle multiline)",
		modelName,
	)
	statusBar.SetText(status)
	
	return statusBar
}

// updateStatusBar updates the status bar content
func (a *App) updateStatusBar() {
	status := fmt.Sprintf(
		"[yellow]Model:[-] %s | [yellow]Commands:[-] Esc (menu), Ctrl+Q (quit), Ctrl+L (toggle multiline) | [yellow]Messages:[-] %d",
		a.currentModel,
		a.chatUI.GetMessageCount(),
	)
	a.statusBar.SetText(status)
}

// handleGlobalKeys handles global keyboard shortcuts
func (a *App) handleGlobalKeys(event *tcell.EventKey) *tcell.EventKey {
	switch {
	case event.Key() == tcell.KeyCtrlQ:
		// Ctrl+Q to quit
		a.Quit()
		return nil
		
	case event.Key() == tcell.KeyCtrlR:
		// Ctrl+R to refresh/clear chat
		a.chatUI.Clear()
		a.updateStatusBar()
		return nil
		
	case event.Key() == tcell.KeyCtrlS:
		// Ctrl+S to show model selector
		a.showModelSelector()
		return nil
	}

	return event
}

// showOptionsMenu displays a context menu with options
func (a *App) showOptionsMenu() {
	modal := tview.NewModal().
		SetText("Choose an action:").
		AddButtons([]string{
			"Change Model (Ctrl+S)",
			"Clear Chat (Ctrl+R)", 
			"Toggle Multiline (Ctrl+L)",
			"Quit (Ctrl+Q)",
			"Cancel",
		}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.pages.RemovePage("options")
			a.app.SetFocus(a.inputField)
			
			switch buttonIndex {
			case 0: // Change Model
				a.showModelSelector()
			case 1: // Clear Chat
				a.chatUI.Clear()
				a.updateStatusBar()
			case 2: // Toggle Multiline
				a.inputField.toggleMultilineMode()
			case 3: // Quit
				a.Quit()
			}
		})

	a.pages.AddPage("options", modal, true, true)
}

// showModelSelector shows the model selection dialog
func (a *App) showModelSelector() {
	// Track selected model ID
	selectedModelID := a.currentModel

	// Create a simple dropdown modal for model selection
	dropdown := tview.NewDropDown().
		SetLabel("Select Model: ")

	// Populate with models
	currentIndex := 0
	for i, model := range models.AvailableModels {
		label := fmt.Sprintf("%s", model.Name)
		dropdown.AddOption(label, nil)
		if model.ID == a.currentModel {
			currentIndex = i
		}
	}
	dropdown.SetCurrentOption(currentIndex)

	// Set up selection handler
	dropdown.SetSelectedFunc(func(text string, index int) {
		if index >= 0 && index < len(models.AvailableModels) {
			selectedModelID = models.AvailableModels[index].ID
		}
	})

	modal := tview.NewModal().
		SetText("Select an AI model:").
		AddButtons([]string{"Select", "Cancel"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.pages.RemovePage("model_select")
			a.app.SetFocus(a.inputField)
			
			if buttonLabel == "Select" {
				a.handleModelSelection(selectedModelID)
			}
		})

	// Create a flex container for the dropdown and modal
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(dropdown, 1, 0, true)

	// Add the flex to the modal
	modalFlex := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(tview.NewBox(), 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(tview.NewBox(), 0, 1, false).
			AddItem(modal, 8, 0, false).
			AddItem(flex, 3, 0, true).
			AddItem(tview.NewBox(), 0, 1, false), 40, 0, true).
		AddItem(tview.NewBox(), 0, 1, false)

	a.pages.AddPage("model_select", modalFlex, true, true)
	a.app.SetFocus(dropdown)
}

// handleModelSelection handles when a new model is selected
func (a *App) handleModelSelection(modelID string) {
	if modelID != a.currentModel {
		a.currentModel = modelID
		
		// Update preferences
		a.preferences.UpdateSelectedModel(modelID)
		
		// Update status bar
		a.updateStatusBar()
		
		// Add system message about model change
		if model, err := models.GetModelByID(modelID); err == nil {
			a.chatUI.AddSystemMessage(fmt.Sprintf("Switched to model: %s", model.Name))
		}
		
		// Notify callback
		if a.onModelChanged != nil {
			a.onModelChanged(modelID)
		}
	}
}

// Run starts the TUI application
func (a *App) Run() error {
	a.updateStatusBar()
	a.app.SetFocus(a.inputField)
	return a.app.Run()
}

// Stop stops the TUI application
func (a *App) Stop() {
	a.app.Stop()
}

// Quit quits the application
func (a *App) Quit() {
	if a.onQuit != nil {
		a.onQuit()
	}
	a.app.Stop()
}

// GetCurrentModel returns the currently selected model ID
func (a *App) GetCurrentModel() string {
	return a.currentModel
}

// AddMessage adds a message to the chat UI
func (a *App) AddUserMessage(content string) {
	a.chatUI.AddUserMessage(content)
	a.updateStatusBar()
}

// AddAssistantMessage adds an assistant message to the chat UI
func (a *App) AddAssistantMessage(content string) {
	a.chatUI.AddAssistantMessage(content)
	a.updateStatusBar()
}

// AddToolMessage adds a tool execution message to the chat UI
func (a *App) AddToolMessage(toolName, args, result string) {
	a.chatUI.AddToolMessage(toolName, args, result)
	a.updateStatusBar()
}

// AddSystemMessage adds a system message to the chat UI
func (a *App) AddSystemMessage(content string) {
	a.chatUI.AddSystemMessage(content)
	a.updateStatusBar()
}

// SetOnSendMessage sets the callback for when a message is sent
func (a *App) SetOnSendMessage(callback func(string)) {
	a.onSendMessage = callback
}

// SetOnModelChanged sets the callback for when the model is changed
func (a *App) SetOnModelChanged(callback func(string)) {
	a.onModelChanged = callback
}

// SetOnQuit sets the callback for when the app is quit
func (a *App) SetOnQuit(callback func()) {
	a.onQuit = callback
}

// ClearChat clears the chat history
func (a *App) ClearChat() {
	a.chatUI.Clear()
	a.updateStatusBar()
}

// FocusInput focuses the input field
func (a *App) FocusInput() {
	a.app.SetFocus(a.inputField)
}