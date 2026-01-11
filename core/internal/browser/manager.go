package browser

import (
	"context"
	"time"

	"github.com/chromedp/chromedp"
)

type BrowserManager struct {
	remoteURL string // e.g. "ws://localhost:9222"
}

func NewBrowserManager(remoteURL string) *BrowserManager {
	return &BrowserManager{remoteURL: remoteURL}
}

// Action encapsulates a browser operation
type Action func(ctx context.Context) error

func (m *BrowserManager) Run(ctx context.Context, actions ...chromedp.Action) error {
	var allocatorCtx context.Context
	var cancel context.CancelFunc

	if m.remoteURL != "" {
		// Connect to existing browser (Remote Debugging)
		allocatorCtx, cancel = chromedp.NewRemoteAllocator(ctx, m.remoteURL)
	} else {
		// Start local headless browser
		allocatorCtx, cancel = chromedp.NewExecAllocator(ctx, chromedp.DefaultExecAllocatorOptions[:]...)
	}
	defer cancel()

	// Create browser context
	browserCtx, cancel := chromedp.NewContext(allocatorCtx)
	defer cancel()

	// Run actions with timeout
	timeoutCtx, cancel := context.WithTimeout(browserCtx, 60*time.Second)
	defer cancel()

	return chromedp.Run(timeoutCtx, actions...)
}

// Screenshot captures a full page screenshot
func (m *BrowserManager) Screenshot(ctx context.Context, url string) ([]byte, error) {
	var buf []byte
	err := m.Run(ctx,
		chromedp.Navigate(url),
		chromedp.FullScreenshot(&buf, 90),
	)
	return buf, err
}

// Navigate opens a URL
func (m *BrowserManager) Navigate(ctx context.Context, url string) error {
	return m.Run(ctx, chromedp.Navigate(url))
}

// Click clicks an element by selector
func (m *BrowserManager) Click(ctx context.Context, url, selector string) error {
	return m.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(selector),
		chromedp.Click(selector),
	)
}

// Type fills an input field
func (m *BrowserManager) Type(ctx context.Context, url, selector, text string) error {
	return m.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(selector),
		chromedp.SendKeys(selector, text),
	)
}
