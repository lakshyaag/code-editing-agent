# TODO List for AI Agent

This document outlines planned improvements for the Go AI agent. The tasks are divided into two main categories: core agent logic and TUI/UX enhancements.

## 🚀 Agent Core Improvements

These tasks focus on making the agent more powerful, efficient, and robust.

-   [x] **Implement Streaming Responses:**
    -   Modify `agent.ProcessMessage` to handle streaming responses from the Gemini API.
    -   This will involve using `GenerateContentStream` instead of `GenerateContent`.
    -   The agent should be able to stream back both text content and tool calls as they are generated. that the order of results sent back to the model matches the order of the tool calls.
-   [ ] **Improve Agent State Management & Communication:**
    -   Define more granular agent states (e.g., `Thinking`, `ExecutingTool`, `AwaitingInput`).
    -   Create a channel for the agent to send state updates to the TUI, so the UI can provide more descriptive feedback than a generic spinner.
-   [ ] **Add Safety and Control Mechanisms:**
    -   Implement a configurable maximum number of turns for the agentic loop to prevent infinite tool-use loops.
    -   Introduce a mechanism for the user to interrupt and stop the agent's execution loop.

## ✨ TUI/UX Improvements

These tasks focus on improving the user experience of the terminal interface.

-   [ ] **Enhance UI to Reflect Agent State:**
    -   Update the TUI to listen for state updates from the agent.
    -   Instead of a generic spinner, display specific status messages like "Executing tool: `file_reader`..." or "Waiting for model response...".
-   [x] **Render Streaming Content:**
    -   Modify the TUI's `Update` function to handle incoming streamed content from the agent.
    -   Update the viewport incrementally as new text arrives, providing a real-time feel.
-   [x] **Create a Structured View for Tool Calls:**
    -   Design a more structured and readable component for displaying tool calls and their results.
    -   Use tables or formatted layouts to clearly show the tool name, arguments, and output.
    -   Render diffs for file modification tools.
-   [ ] **Implement User Confirmation for Tools:**
    -   Introduce a confirmation step in the TUI for potentially destructive tool operations (e.g., `edit_file`, `delete_file`).
    -   The agent loop should pause and wait for the user's `(y/n)` response before proceeding.
-   [x] **Improve Message Display:**
    -   Use a markdown renderer (like `glamour`) to render the agent's responses, allowing for better formatting of code blocks, lists, and other elements.
-   [ ] **Refactor Message Rendering:**
    -   The `renderConversation` function re-renders everything on each change. Explore more efficient rendering strategies or components to improve performance for long conversations.
    -   Improve how collapsible sections are handled, perhaps by making each message its own Bubble Tea `model`.
-   [ ] **Add a Command Palette:**
    -   Implement a command palette to provide quick access to meta-actions like clearing history (`/clear`), quitting (`/quit`), or copying the last response (`/copy`). 