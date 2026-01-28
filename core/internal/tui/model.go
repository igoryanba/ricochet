package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/igoryan-dao/ricochet/internal/agent"
	"github.com/igoryan-dao/ricochet/internal/config"
	"github.com/igoryan-dao/ricochet/internal/livemode"
	"github.com/igoryan-dao/ricochet/internal/protocol"
	"github.com/igoryan-dao/ricochet/internal/tui/style"
)

var WhimsicalVerbs = []string{
	"Perambulating",
	"Reticulating",
	"Puzzling",
	"Sussing",
	"Meandering",
	"Cogitating",
	"Ruminating",
}

// -- Messages --

type LogMsg struct {
	Level string
	Text  string
}

type StreamMsg struct {
	Content string
	Done    bool
}

type ThoughtsMsg struct {
	Content string
}

type ErrorMsg struct {
	Err error
}

type AskUserMsg struct {
	Question string
	RespChan chan string
	IsInput  bool
	Diff     string // Optional diff for review
}

type AskUserChoiceMsg struct {
	Question string
	Choices  []string
	RespChan chan int
}

type SlashCmdResMsg struct {
	Command  string
	Response string
	Error    error
}

type RemoteInputMsg struct {
	Content string
}

type ReloadEtherMsg struct {
	Token string
}

type EtherToggleDoneMsg struct{}

type RemoteChatMsg struct {
	Message agent.ChatMessage
}

type DemoUpdateMsg func(*Model)

type TaskNode struct {
	ID         string
	ParentID   string
	Name       string
	Status     string // "running", "done", "failed"
	Children   []*TaskNode
	Meta       string // e.g. "19 tools used"
	Result     string // Tool output (for terminal block)
	Expanded   bool
	Depth      int
	AgentName  string // e.g. "ARCH", "QA"
	AgentColor string // Hex color
}

// -- Interleaved Blocks Architecture --
// Replaces monolithic TaskTree with block-based history for Claude Code-style rendering

type BlockType int

const (
	BlockUserQuery BlockType = iota // User input message
	BlockAgentText                  // Agent text response
	BlockAgentTree                  // Agent tool execution tree
)

type HistoryBlock struct {
	Type      BlockType
	Content   string      // For Text/User blocks
	Reasoning string      // For DeepSeek reasoning
	TaskTree  []*TaskNode // For Tree blocks (isolated tree for this sequence)
	IsActive  bool        // Only the last block can be active (spinning)
}

// -- Model --

type Model struct {
	Cwd        string
	Controller *agent.Controller
	SessionID  string
	MsgChan    chan tea.Msg
	ModelName  string

	Viewport  viewport.Model
	Textarea  textarea.Model
	Spinner   spinner.Model
	IsLoading bool
	Thoughts  string

	// Styling
	Renderer *glamour.TermRenderer

	// Approval flow
	PendingApproval *AskUserMsg
	PendingChoice   *AskUserChoiceMsg
	ConfirmationIdx int

	// Interleaved Blocks (Claude Code-style rendering)
	// Each block is either User Query, Agent Text, or Agent Tree
	// This is now the Single Source of Truth for history.
	Blocks []*HistoryBlock

	// Deduplication State
	// Tracks how many steps have been rendered for each TaskName to prevent "Snowball Effect"
	RenderedSteps map[string]int

	// Modes
	IsPlanMode     bool
	PlanCursor     int  // Index of selected task in plan view
	PlanAddingTask bool // True if typing new task
	IsShellFocused bool // Tab toggles between Input and Shell (Viewport) focus

	// Task Progress (Legacy map - might deplete in favor of Tree, but keeping for compatibility)
	Tasks map[string]*protocol.TaskProgress

	// Autocomplete
	AllCommands        []string
	Suggestions        []string
	SelectedSuggestion int
	ShowSuggestions    bool

	// Stats
	TokenUsage   int
	CurrentModel string

	TerminalWidth  int
	TerminalHeight int

	// Ether Mode
	LiveCtrl      *livemode.Controller
	IsEtherMode   bool
	IsEtherActive bool
	IsToggling    bool
	SettingsStore *config.Store
	CurrentAction string // Granular status like "Searching...", "Writing..."

	// Whimsical Flavor
	CurrentStatusStr string // "Meandering..."
	StatusTick       int

	// Auto-Pilot (Autonomous Agent)
	AutoStepsRemaining int
}

func NewModel(cwd, modelName string, msgChan chan tea.Msg, ctrl *agent.Controller) Model {
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
		glamour.WithColorProfile(termenv.ANSI), // Force 16-color mode to prevent 256-color artifacts
	)

	ta := textarea.New()
	ta.Placeholder = "Ask Ricochet..."
	ta.Focus()
	ta.Prompt = "" // Removed vertical bar
	ta.CharLimit = 0
	ta.SetHeight(1) // Start with 1 line
	ta.ShowLineNumbers = false

	// Custom Styles to remove black background artifact
	// bubbles v0.21.0 uses FocusedStyle/BlurredStyle
	ta.FocusedStyle.Placeholder = ta.FocusedStyle.Placeholder.Background(lipgloss.NoColor{}).Foreground(style.MutedGray)
	ta.BlurredStyle.Placeholder = ta.BlurredStyle.Placeholder.Background(lipgloss.NoColor{}).Foreground(style.MutedGray)
	ta.FocusedStyle.CursorLine = ta.FocusedStyle.CursorLine.Background(lipgloss.NoColor{})
	ta.BlurredStyle.CursorLine = ta.BlurredStyle.CursorLine.Background(lipgloss.NoColor{})

	vp := viewport.New(80, 20)
	// Welcome content will be set in Init or View, or we can helper it here.
	// We'll leave it empty initially or set it via a helper.

	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = style.SpinnerStyle

	session := ctrl.CreateSession()

	// Initial commands list
	cmds := []string{
		"/help", "/model", "/status", "/checkpoint", "/restore", "/init", "/shell",
		"/memory", "/hooks", "/clear", "/mode", "/exit", "/ether", "/permissions",
	}

	// Generate Welcome Content (Plain Text to prevent ALL artifacts)
	// We avoid all coloring for the welcome message to be safe.

	welcome := fmt.Sprintf(`Welcome to Ricochet v0.1.0
Model: %s
CWD: %s

Type /help for commands.
Type ? for shortcuts.
`,
		modelName,
		cwd,
	)

	vp.SetContent(welcome)

	return Model{
		Cwd:        cwd,
		Controller: ctrl,
		SessionID:  session.ID,
		MsgChan:    msgChan,
		ModelName:  modelName,

		Viewport:    vp,
		Textarea:    ta,
		Spinner:     sp,
		Renderer:    renderer,
		AllCommands: cmds,

		// Blocks initialized with welcome message
		Blocks: []*HistoryBlock{
			{
				Type:    BlockAgentText,
				Content: welcome,
			},
		},

		RenderedSteps: make(map[string]int),
		Tasks:         make(map[string]*protocol.TaskProgress),

		// Default to Chat Mode
		IsPlanMode: false,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		tea.EnableMouseCellMotion, // Enable mouse support for scrolling
		m.Spinner.Tick,
		m.waitForMsg(),
	)
}

func (m Model) waitForMsg() tea.Cmd {
	return func() tea.Msg {
		return <-m.MsgChan
	}
}
