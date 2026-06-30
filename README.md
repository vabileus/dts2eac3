<div align="center">

# 🎬 dts2eac3

**Batch-convert DTS audio tracks in your `.mkv` files to E-AC3 — fast, pretty, dependency-free.**

A zero-dependency Go CLI with a live terminal UI, native desktop notifications, and parallel processing. Ported from a humble Windows `.bat` script.

<br>

[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Platforms](https://img.shields.io/badge/platforms-Windows%20%7C%20macOS%20%7C%20Linux-555)](#%EF%B8%8F-platform-support)
[![Dependencies](https://img.shields.io/badge/dependencies-0-brightgreen)](#-why-zero-dependencies)
[![ffmpeg](https://img.shields.io/badge/requires-ffmpeg-007808?logo=ffmpeg&logoColor=white)](https://ffmpeg.org)
[![License](https://img.shields.io/badge/license-MIT-blue)](#-license)

</div>

---

## ✨ Features

- 🎯 **Smart detection** — probes every audio stream via `ffprobe` (JSON), converts only DTS tracks, copies everything else (video, subtitles, other audio) untouched.
- 📊 **Live progress** — real-time per-file progress bars driven by `ffmpeg -progress`, not a silent black box.
- ⚡ **Parallel** — a worker pool converts multiple files at once (`-jobs N`).
- 🔔 **Native notifications** — a real desktop toast when the batch finishes (Windows / macOS / Linux).
- 🧠 **Idempotent** — skips files with no DTS and files already converted, so re-running on the same folder is safe.
- 🪶 **Zero dependencies** — pure Go standard library. `go build` works offline; cross-compiles anywhere.
- 🧹 **Safe** — `Ctrl-C` cancels the running `ffmpeg` and removes the half-written output.

---

## 📦 Installation

You need **`ffmpeg`** and **`ffprobe`** on your `PATH` (or pass them explicitly).

### Build from source

```bash
git clone https://github.com/yourname/dts2eac3.git
cd dts2eac3
go build -o dts2eac3 ./...
```

### Cross-compile for Windows from any OS

```bash
GOOS=windows GOARCH=amd64 go build -o dts2eac3.exe ./...
```

---

## 🚀 Usage

Run it **from inside the folder with your movies** (or point it anywhere with `-dir`):

```bash
# scan current folder, write results to ./converted
dts2eac3

# two files at a time, custom bitrate and directories
dts2eac3 -dir "D:\Movies\Season 1" -out "D:\Movies\eac3" -jobs 2 -bitrate 1024k

# analyze only — no conversion
dts2eac3 -dry-run
```

> [!TIP]
> On Windows, **don't double-click the `.exe`** — open a terminal *in the movie folder*
> (Shift + Right-click → "Open PowerShell window here") and run it there, or just use the
> `-dir` flag from anywhere.

### Output

```text
  ⏵ dts2eac3  ·  DTS → E-AC3 batch converter
    dir=.  out=converted  bitrate=1536k  jobs=2  files=3

  ✓ queue movie_one.mkv — a0 dts→eac3, a1 ac3 copy
  ✓ queue movie_two.mkv — a0 dts→eac3
  ∅ skip  no_dts.mkv — no DTS (aac)

  Converting 2 file(s) with 2 worker(s)
  ⠹  47% ████████████░░░░░░░░░░░░ movie_one        3s
  ⠼  31% ████████░░░░░░░░░░░░░░░░ movie_two        2s
  ✔ done  movie_one — 12s

  Summary
  ───────
  Converted              2
  Failed                 0
  Skipped (no DTS)       1
  Skipped (exists)       0
```

---

## ⚙️ Flags

| Flag          | Default           | Description                              |
|---------------|-------------------|------------------------------------------|
| `-dir`        | `.`               | Directory to scan for input files        |
| `-out`        | `converted`       | Output directory                         |
| `-ext`        | `mkv`             | Input file extension (without the dot)   |
| `-bitrate`    | `1536k`           | E-AC3 target bitrate                      |
| `-jobs`       | `1`               | Number of files to convert in parallel   |
| `-ffmpeg`     | `ffmpeg`          | Path to the ffmpeg binary                |
| `-ffprobe`    | `ffprobe`         | Path to the ffprobe binary               |
| `-log`        | `convert_log.txt` | Log file path                            |
| `-no-color`   | `false`           | Disable ANSI colors                      |
| `-no-notify`  | `false`           | Disable desktop notifications            |
| `-dry-run`    | `false`           | Analyze only, don't convert              |

---

## 🖥️ Platform support

| OS       | Notifications                                   | ANSI colors                          |
|----------|-------------------------------------------------|--------------------------------------|
| Windows  | Native toast (WinRT `ToastNotificationManager`) | Auto-enabled VT on Win10+ consoles   |
| macOS    | `osascript display notification`                | Native                               |
| Linux    | `notify-send` (libnotify)                       | Native                               |

Notifications are best-effort — if the mechanism is unavailable, conversion never fails.

---

## 🧩 How it works

```text
ffmpeg -y -i input.mkv -map 0 -c copy \
       -c:a:0 eac3 -b:a:0 1536k -metadata:s:a:0 title=E-AC3 \   # DTS track → E-AC3
       -c:a:1 copy \                                            # everything else copied
       -progress pipe:1 -nostats output.mkv
```

1. `ffprobe -of json` reports each audio stream's codec and the file duration.
2. Streams whose codec contains `dts` are re-encoded to `eac3`; the rest are stream-copied.
3. `ffmpeg`'s `-progress pipe:1` feed is parsed (`out_time_us` ÷ duration) to render the live bar.
4. A summary is printed, written to the log, and pushed as a desktop notification.

---

## 🗂️ Project structure

```text
dts2eac3/
├── main.go          # flags, orchestration, worker pool, live dashboard
├── probe.go         # ffprobe wrapper, JSON parsing, copy/convert decision
├── convert.go       # ffmpeg invocation, -progress parsing
├── ui.go            # ANSI palette, spinner, progress bar, live region renderer
├── logger.go        # thread-safe file logger
├── notify.go        # notification dispatcher (respects -no-notify)
├── notify_windows.go / notify_darwin.go / notify_linux.go   # per-OS, build-tagged
├── vt_windows.go / vt_other.go   # enable ANSI on Windows consoles via syscall
└── helpers.go       # banner, summary table, formatting
```

---

## 🪶 Why zero dependencies?

The whole tool is built on the Go standard library — no `pterm`, no `beeep`, nothing to `go get`:

- `go build` works **offline** and in locked-down CI/sandbox environments.
- **Cross-compilation just works** for every OS out of one source tree.
- The terminal rendering, native notifications, and Windows VT-mode setup are all done by hand — which makes this a decent reference for those patterns in Go.

---

## 📜 License

MIT © vabileus
