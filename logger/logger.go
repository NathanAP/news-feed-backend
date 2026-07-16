package logger

import (
	"fmt"
	"os"
	"time"
)

type Color int

const (
	ColorBlue   Color = 34
	ColorGreen  Color = 32
	ColorRed    Color = 31
	ColorYellow Color = 33
	ColorCyan   Color = 36
	ColorWhite  Color = 37
)

var enabled bool

// Setup initializes the logger. Must be called once after loading environment variables.
func Setup() {
	enabled = os.Getenv("VERBOSE_MODE") == "true"
}

// Log prints a colored message to stdout when VERBOSE_MODE is true.
// Accepts any value: primitives are printed directly, complex types via %+v.
// Color defaults to blue if not provided.
func Log(message any, color ...Color) {
	if !enabled {
		return
	}

	c := ColorBlue
	if len(color) > 0 {
		c = color[0]
	}

	var text string
	switch v := message.(type) {
	case string:
		text = v
	case error:
		text = v.Error()
	default:
		text = fmt.Sprintf("%+v", v)
	}

	fmt.Printf("\033[%dm%s\033[0m\n", int(c), text)
}

// Print writes a colored message to stdout unconditionally, ignoring the global VERBOSE_MODE
// gate. Use it for output controlled by its own dedicated flag (e.g. the discovery CRON, gated
// by RSS_FEED_CRON_VERBOSE_MODE). Color defaults to blue.
func Print(message any, color ...Color) {
	c := ColorBlue
	if len(color) > 0 {
		c = color[0]
	}

	var text string
	switch v := message.(type) {
	case string:
		text = v
	case error:
		text = v.Error()
	default:
		text = fmt.Sprintf("%+v", v)
	}

	fmt.Printf("\033[%dm%s\033[0m\n", int(c), text)
}

// RouteStart logs a route entry marker. Call at the beginning of each handler.
func RouteStart(path string) {
	Log(fmt.Sprintf("@@@ ROUTE START - %s - %s @@@", path, now()), ColorYellow)
}

// RouteEnd logs a route exit marker. Pair with RouteStart via defer.
func RouteEnd(path string) {
	Log(fmt.Sprintf("@@@ ROUTE END - %s - %s @@@", path, now()), ColorYellow)
}

// now stamps log lines. Explicitly UTC so the logs agree with the timestamps the API returns and with
// what is stored, instead of following whatever timezone the host happens to be set to.
func now() string {
	return time.Now().UTC().Format("2006-01-02 15:04:05")
}
