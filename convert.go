package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// buildArgs assembles the ffmpeg argument list for one job.
//
// It mirrors the original .bat:
//
//	ffmpeg -y -i IN -map 0 -c copy
//	       [per audio stream: copy | eac3 + bitrate + title metadata]
//	       -progress pipe:1 -nostats OUT
func buildArgs(j Job, bitrate, outPath string) []string {
	args := []string{"-y", "-i", j.Path, "-map", "0", "-c", "copy"}
	for _, a := range j.Audio {
		i := strconv.Itoa(a.Index)
		switch a.Action {
		case ActionConvert:
			args = append(args,
				"-c:a:"+i, "eac3",
				"-b:a:"+i, bitrate,
				"-metadata:s:a:"+i, "title=E-AC3",
			)
		default:
			args = append(args, "-c:a:"+i, "copy")
		}
	}
	// Machine-readable progress on stdout; suppress the noisy default stats.
	args = append(args, "-progress", "pipe:1", "-nostats", outPath)
	return args
}

// convert runs ffmpeg for a single job, streaming progress to onProgress
// (a fraction in [0,1]). It returns the tail of ffmpeg's stderr on failure.
func convert(ctx context.Context, ffmpegBin, bitrate string, j Job, onProgress func(float64)) error {
	args := buildArgs(j, bitrate, j.OutPath)
	cmd := exec.CommandContext(ctx, ffmpegBin, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	// Keep stderr for diagnostics; ffmpeg logs everything human-facing there.
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	// Parse the -progress key=value stream line by line.
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		key, val, ok := strings.Cut(scanner.Text(), "=")
		if !ok {
			continue
		}
		switch key {
		case "out_time_us", "out_time_ms":
			if j.Duration > 0 {
				if d := parseProgressTime(key, val); d >= 0 {
					frac := float64(d) / float64(j.Duration)
					onProgress(clamp01(frac))
				}
			}
		case "progress":
			if val == "end" {
				onProgress(1)
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ffmpeg: %w\n%s", err, lastLines(stderr.String(), 4))
	}
	return nil
}

// parseProgressTime converts ffmpeg's out_time_us/out_time_ms field to a
// duration. Note ffmpeg's "out_time_ms" is historically microseconds, so both
// keys carry microseconds in practice; we handle the unit explicitly.
func parseProgressTime(key, val string) time.Duration {
	n, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
	if err != nil || n < 0 {
		return -1
	}
	// Both keys are microseconds in current ffmpeg builds.
	return time.Duration(n) * time.Microsecond
}

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

func lastLines(s string, n int) string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return "    " + strings.Join(lines, "\n    ")
}

// removeFile deletes a (partial) output, ignoring "not exists".
func removeFile(path string) {
	_ = os.Remove(path)
}
