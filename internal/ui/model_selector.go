package ui

import (
	"agent/internal/models"
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ModelSelector manages the model selection interface
type ModelSelector struct {
	*tview.Modal
	dropdown        *tview.DropDown
	detailsView     *tview.TextView
	selectedModelID string
	onModelSelected func(modelID string)
}

// NewModelSelector creates a new model selector component
func NewModelSelector(currentModelID string, onModelSelected func(string)) *ModelSelector {
	// Create dropdown with model options
	dropdown := tview.NewDropDown().
		SetLabel("Select Model: ").
		SetFieldWidth(50)

	// Create details view
	detailsView := tview.NewTextView()
	detailsView.SetDynamicColors(true)
	detailsView.SetWordWrap(true)
	detailsView.SetBorder(true)
	detailsView.SetTitle(" Model Details ")

	// Populate dropdown with available models
	currentIndex := 0
	for i, model := range models.AvailableModels {
		label := fmt.Sprintf("%s - %s", model.Name, model.Description)
		if len(label) > 47 {
			label = label[:44] + "..."
		}
		dropdown.AddOption(label, nil)
		
		if model.ID == currentModelID {
			currentIndex = i
		}
	}

	// Create modal for the selector
	modal := tview.NewModal().
		SetText("Select a Gemini model for your conversations:").
		AddButtons([]string{"Select", "Cancel"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			// This will be handled by the parent app
		})

	// Create the selector struct
	selector := &ModelSelector{
		Modal:           modal,
		dropdown:        dropdown,
		detailsView:     detailsView,
		selectedModelID: currentModelID,
		onModelSelected: onModelSelected,
	}

	// Set up dropdown selection handler
	dropdown.SetSelectedFunc(func(text string, index int) {
		if index < len(models.AvailableModels) {
			model := models.AvailableModels[index]
			selector.selectedModelID = model.ID
			
			details := fmt.Sprintf(
				"[yellow]Model ID:[-] %s\n\n"+
				"[yellow]Max Tokens:[-] %s\n\n"+
				"[yellow]Description:[-] %s",
				model.ID,
				formatTokens(model.MaxTokens),
				model.Description,
			)
			selector.detailsView.SetText(details)
		}
	})

	// Set current option and initial details
	dropdown.SetCurrentOption(currentIndex)
	if currentIndex < len(models.AvailableModels) {
		model := models.AvailableModels[currentIndex]
		details := fmt.Sprintf(
			"[yellow]Model ID:[-] %s\n\n"+
			"[yellow]Max Tokens:[-] %s\n\n"+
			"[yellow]Description:[-] %s",
			model.ID,
			formatTokens(model.MaxTokens),
			model.Description,
		)
		selector.detailsView.SetText(details)
	}

	return selector
}

// Show displays the model selector
func (m *ModelSelector) Show(app *tview.Application, pages *tview.Pages) {
	// Create the main content
	content := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(tview.NewBox(), 0, 1, false). // Top spacer
		AddItem(m.createContent(), 12, 0, false). // Main content
		AddItem(tview.NewBox(), 0, 1, false)  // Bottom spacer

	mainFlex := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(tview.NewBox(), 0, 1, false). // Left spacer
		AddItem(content, 80, 0, true).        // Main content
		AddItem(tview.NewBox(), 0, 1, false)  // Right spacer

	pages.AddPage("model_selector", mainFlex, true, true)
	app.SetFocus(m.dropdown)
}

// createContent creates the content for the model selector
func (m *ModelSelector) createContent() tview.Primitive {
	// Create the dropdown
	m.dropdown.SetBorder(true).SetTitle(" Model Selection ")

	// Create buttons
	buttonFlex := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(tview.NewBox(), 0, 1, false). // Spacer
		AddItem(m.createButton("Select", tcell.ColorGreen, func() {
			if m.onModelSelected != nil {
				m.onModelSelected(m.selectedModelID)
			}
		}), 12, 0, false).
		AddItem(tview.NewBox(), 2, 0, false). // Spacer
		AddItem(m.createButton("Cancel", tcell.ColorRed, func() {
			// Close modal without selecting
		}), 12, 0, false).
		AddItem(tview.NewBox(), 0, 1, false) // Spacer

	// Layout the content
	content := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(m.dropdown, 3, 0, true).
		AddItem(tview.NewBox().SetBorder(false), 1, 0, false). // Spacer
		AddItem(m.detailsView, 0, 1, false).
		AddItem(tview.NewBox().SetBorder(false), 1, 0, false). // Spacer
		AddItem(buttonFlex, 3, 0, false)

	return content
}

// createButton creates a styled button
func (m *ModelSelector) createButton(text string, color tcell.Color, selected func()) tview.Primitive {
	button := tview.NewButton(text)
	button.SetSelectedFunc(selected)
	return button
}

// GetSelectedModelID returns the currently selected model ID
func (m *ModelSelector) GetSelectedModelID() string {
	return m.selectedModelID
}

// formatTokens formats the token count for display
func formatTokens(tokens int) string {
	if tokens >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(tokens)/1000000)
	} else if tokens >= 1000 {
		return fmt.Sprintf("%.1fK", float64(tokens)/1000)
	}
	return fmt.Sprintf("%d", tokens)
}

// Hide hides the model selector
func (m *ModelSelector) Hide(pages *tview.Pages) {
	pages.RemovePage("model_selector")
}