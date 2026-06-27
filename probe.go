package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// AudioAction is what we decide to do with a single audio stream.
type AudioAction int

const (
	ActionCopy AudioAction = iota
	ActionConvert
)

// AudioStream describes one audio track and the action we'll take on it.
// Index is the audio-relative index (0-based), i.e. the N in ffmpeg's -c:a:N.
type AudioStream struct {
	Index  int
	Codec  string
	Action AudioAction
}

// ProbeResult is the outcome of inspecting a single media file.
type ProbeResult struct {
	Streams  []AudioStream
	Duration time.Duration
	HasDTS   bool
}

// ffprobe JSON shape (only the fields we care about).
type ffprobeJSON struct {
	Streams []struct {
		CodecName string `json:"codec_name"`
		CodecType string `json:"codec_type"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

// probe runs ffprobe once and returns the audio layout + duration of a file.
func probe(ctx context.Context, ffprobeBin, path string) (ProbeResult, error) {
	cmd := exec.CommandContext(ctx, ffprobeBin,
		"-v", "error",
		"-select_streams", "a",
		"-show_entries", "stream=codec_name,codec_type:format=duration",
		"-of", "json",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return ProbeResult{}, fmt.Errorf("ffprobe failed: %w", asExecErr(err))
	}

	var raw ffprobeJSON
	if err := json.Unmarshal(out, &raw); err != nil {
		return ProbeResult{}, fmt.Errorf("parsing ffprobe output: %w", err)
	}

	res := ProbeResult{Duration: parseSeconds(raw.Format.Duration)}
	idx := 0
	for _, s := range raw.Streams {
		if s.CodecType != "audio" {
			continue
		}
		action := ActionCopy
		if isDTS(s.CodecName) {
			action = ActionConvert
			res.HasDTS = true
		}
		res.Streams = append(res.Streams, AudioStream{
			Index:  idx,
			Codec:  s.CodecName,
			Action: action,
		})
		idx++
	}
	return res, nil
}

// isDTS matches the bat's case-insensitive substring check on "dts"
// (covers "dts", "dts-hd", etc.).
func isDTS(codec string) bool {
	return strings.Contains(strings.ToLower(codec), "dts")
}

func parseSeconds(s string) time.Duration {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || f <= 0 {
		return 0
	}
	return time.Duration(f * float64(time.Second))
}

// asExecErr enriches exec errors with stderr when available.
func asExecErr(err error) error {
	if ee, ok := err.(*exec.ExitError); ok {
		msg := strings.TrimSpace(string(ee.Stderr))
		if msg != "" {
			return fmt.Errorf("%v: %s", ee, lastLine(msg))
		}
	}
	return err
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return lines[len(lines)-1]
}
