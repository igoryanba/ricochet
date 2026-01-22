package style

import "github.com/charmbracelet/lipgloss"

// Colors
var (
	BurntOrange = lipgloss.Color("#DA702C") // Adjusted to match screenshot Coral
	MutedGray   = lipgloss.Color("245")
	White       = lipgloss.Color("#FFFFFF")
	Black       = lipgloss.Color("#000000")
	Pink        = lipgloss.Color("205")
	Cyan        = lipgloss.Color("86")
	Red         = lipgloss.Color("196")
	Green       = lipgloss.Color("#2E8B57") // SeaGreen for checkmarks
)

// Bullets
var (
	BulletUser   = ">" // Matches screenshot input history
	BulletAgent  = "●" // Matches screenshot agent message
	BulletSystem = "○"
	BulletError  = "x"
	BulletTask   = "●" // Root task icon
)

// Base Styles
var (
	UserStyle   = lipgloss.NewStyle().Foreground(White) // Not bold, just white
	AgentStyle  = lipgloss.NewStyle().Foreground(BurntOrange)
	SystemStyle = lipgloss.NewStyle().Foreground(MutedGray)
	ErrorStyle  = lipgloss.NewStyle().Foreground(Red)
	TaskStyle   = lipgloss.NewStyle().Foreground(MutedGray)

	SpinnerStyle = lipgloss.NewStyle().Foreground(BurntOrange)

	// Mode Styles
	PlanStyle = lipgloss.NewStyle().Foreground(Cyan).Bold(true)
	ActStyle  = lipgloss.NewStyle().Foreground(BurntOrange).Bold(true)

	// Thinking / Status
	ThinkingStyle = lipgloss.NewStyle().Foreground(Red)       // "Tinkering..." is reddish
	MetaStyle     = lipgloss.NewStyle().Foreground(MutedGray) // "(10s · 143 tokens)"

	// Warning/Gate
	Yellow       = lipgloss.Color("#F1C40F")
	WarningStyle = lipgloss.NewStyle().Foreground(Yellow).Bold(true)
)

// Component Styles
var (
	HeaderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BurntOrange).
			Padding(0, 1).
			Foreground(White)

	HeaderLabelStyle = lipgloss.NewStyle().
				Foreground(BurntOrange).
				Bold(true)

	FooterStyle = lipgloss.NewStyle().
			Foreground(MutedGray)

	TreeStyle = lipgloss.NewStyle().
			Foreground(MutedGray)

	TreeActiveStyle = lipgloss.NewStyle().
			Foreground(BurntOrange)

	// Box Styles
	BorderColor = lipgloss.NewStyle().Foreground(BurntOrange)
	BoxStyle    = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor.GetForeground()).
			Padding(0, 1)

	TitleStyle = lipgloss.NewStyle().Foreground(BorderColor.GetForeground()).Bold(true)
)
