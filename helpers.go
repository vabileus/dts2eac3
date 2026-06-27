package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ensureBinaries verifies ffmpeg/ffprobe are resolvable.
func ensureBinaries(ffmpeg, ffprobe string) error {
	for _, bin := range []string{ffmpeg, ffprobe} {
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("%q not found in PATH (install ffmpeg or pass -ffmpeg/-ffprobe)", bin)
		}
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// codecList joins codec names for display, e.g. "ac3, aac".
func codecList(streams []AudioStream) string {
	names := make([]string, len(streams))
	for i, s := range streams {
		names[i] = s.Codec
	}
	return strings.Join(names, ", ")
}

// decisionString describes per-stream actions with color.
func decisionString(p palette, streams []AudioStream) string {
	parts := make([]string, len(streams))
	for i, s := range streams {
		tag := fmt.Sprintf("a%d", s.Index)
		if s.Action == ActionConvert {
			parts[i] = tag + " " + p.green(s.Codec+"→eac3")
		} else {
			parts[i] = tag + " " + p.dim(s.Codec+" copy")
		}
	}
	return strings.Join(parts, ", ")
}

func decisionStringPlain(streams []AudioStream) string {
	parts := make([]string, len(streams))
	for i, s := range streams {
		verb := "copy"
		if s.Action == ActionConvert {
			verb = "→eac3"
		}
		parts[i] = fmt.Sprintf("a%d %s %s", s.Index, s.Codec, verb)
	}
	return strings.Join(parts, ", ")
}

// ---- summary -----------------------------------------------------------

type summary struct {
	ok            int
	failed        int
	skippedExists int
	skippedNoDTS  int
	failedNames   []string
}

func summarize(results []result, skipExists, skipNoDTS int) summary {
	s := summary{skippedExists: skipExists, skippedNoDTS: skipNoDTS}
	for _, r := range results {
		if r.err != nil {
			s.failed++
			s.failedNames = append(s.failedNames, r.name)
		} else {
			s.ok++
		}
	}
	return s
}

func renderSummary(p palette, s summary, jobs []Job) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(p.bold("  Summary\n"))
	b.WriteString(p.dim("  ───────\n"))
	row := func(label, val, color string) {
		colored := val
		switch color {
		case "green":
			colored = p.green(val)
		case "red":
			colored = p.red(val)
		case "yellow":
			colored = p.yellow(val)
		case "dim":
			colored = p.dim(val)
		}
		fmt.Fprintf(&b, "  %-22s %s\n", label, colored)
	}
	row("Converted", fmt.Sprintf("%d", s.ok), pick(s.ok > 0, "green", "dim"))
	row("Failed", fmt.Sprintf("%d", s.failed), pick(s.failed > 0, "red", "dim"))
	row("Skipped (no DTS)", fmt.Sprintf("%d", s.skippedNoDTS), "dim")
	row("Skipped (exists)", fmt.Sprintf("%d", s.skippedExists), "yellow")
	if len(s.failedNames) > 0 {
		b.WriteString(p.red("  Failed files: ") + strings.Join(s.failedNames, ", ") + "\n")
	}
	return b.String()
}

func notificationFor(s summary) (string, string) {
	title := "DTS → E-AC3 conversion finished"
	if s.failed > 0 {
		title = "DTS → E-AC3 finished with errors"
	}
	body := fmt.Sprintf("%d converted, %d failed, %d skipped",
		s.ok, s.failed, s.skippedExists+s.skippedNoDTS)
	return title, body
}

// ---- banner & formatting ----------------------------------------------

func printBanner(r *Renderer, cfg config, n int) {
	p := r.pal
	title := "  ⏵ dts2eac3  ·  DTS → E-AC3 batch converter"
	meta := fmt.Sprintf("    dir=%s  out=%s  bitrate=%s  jobs=%d  files=%d",
		cfg.dir, cfg.out, cfg.bitrate, cfg.jobs, n)
	r.Plain(p.bold(p.cyan(title)) + "\n" + p.dim(meta) + "\n")
}

func fmtDur(d time.Duration) string {
	d = d.Round(time.Second)
	m := int(d / time.Minute)
	s := int((d % time.Minute) / time.Second)
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s + strings.Repeat(" ", max-len(r))
	}
	return string(r[:max-1]) + "…"
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func pick(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
