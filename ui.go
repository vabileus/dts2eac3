package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// ---- color palette -----------------------------------------------------

type palette struct{ enabled bool }

func (p palette) wrap(code, s string) string {
	if !p.enabled {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func (p palette) dim(s string) string     { return p.wrap("2", s) }
func (p palette) bold(s string) string    { return p.wrap("1", s) }
func (p palette) red(s string) string     { return p.wrap("31", s) }
func (p palette) green(s string) string   { return p.wrap("32", s) }
func (p palette) yellow(s string) string  { return p.wrap("33", s) }
func (p palette) blue(s string) string    { return p.wrap("34", s) }
func (p palette) magenta(s string) string { return p.wrap("35", s) }
func (p palette) cyan(s string) string    { return p.wrap("36", s) }

// ---- spinner & bar -----------------------------------------------------

var spinnerFrames = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

func spinnerFrame(t time.Time) string {
	i := (t.UnixNano() / int64(80*time.Millisecond)) % int64(len(spinnerFrames))
	return string(spinnerFrames[i])
}

// progressBar renders a fixed-width [████░░░░] bar for a fraction in [0,1].
func progressBar(frac float64, width int) string {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	filled := int(frac*float64(width) + 0.5)
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// ---- live region renderer ---------------------------------------------

// Renderer serializes all terminal output through a mutex. It keeps a "live
// region" (the bottom N lines) that can be repainted in place, while Log()
// pushes permanent lines above it that scroll normally.
type Renderer struct {
	mu       sync.Mutex
	out      io.Writer
	tty      bool
	pal      palette
	liveRows int // how many rows the live region currently occupies
}

func NewRenderer(out *os.File, color bool) *Renderer {
	tty := isTerminal(out)
	return &Renderer{
		out: out,
		tty: tty,
		pal: palette{enabled: color && tty},
	}
}

// clearLive moves the cursor to the top of the live region and erases it.
func (r *Renderer) clearLive() {
	if r.liveRows == 0 {
		return
	}
	fmt.Fprintf(r.out, "\x1b[%dA", r.liveRows) // cursor up N rows
	fmt.Fprint(r.out, "\x1b[0J")               // clear from cursor to end
	r.liveRows = 0
}

// SetLive replaces the live region with the given lines (no-op on non-tty).
func (r *Renderer) SetLive(lines []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.tty {
		return
	}
	r.clearLive()
	for _, ln := range lines {
		fmt.Fprint(r.out, ln, "\n")
	}
	r.liveRows = len(lines)
}

// Log prints a permanent line above the live region.
func (r *Renderer) Log(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clearLive()
	fmt.Fprintln(r.out, line)
}

// Plain prints raw lines (used for banners/summaries), clearing live first.
func (r *Renderer) Plain(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clearLive()
	fmt.Fprint(r.out, s)
	if !strings.HasSuffix(s, "\n") {
		fmt.Fprintln(r.out)
	}
}

// isTerminal reports whether f is an interactive character device.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
