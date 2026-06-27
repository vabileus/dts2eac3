package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Job is a single file scheduled for conversion.
type Job struct {
	Path     string
	Name     string // base name without extension
	OutPath  string
	Audio    []AudioStream
	Duration time.Duration
}

// result of processing one job.
type result struct {
	name string
	err  error // nil = success
	took time.Duration
}

type config struct {
	dir      string
	out      string
	ext      string
	bitrate  string
	jobs     int
	ffmpeg   string
	ffprobe  string
	logPath  string
	noColor  bool
	noNotify bool
	dryRun   bool
}

func main() {
	cfg := parseFlags()
	enableVirtualTerminal()

	r := NewRenderer(os.Stdout, !cfg.noColor)
	notifier := Notifier{enabled: !cfg.noNotify}

	if err := run(cfg, r, notifier); err != nil {
		r.Plain(r.pal.red("error: ") + err.Error())
		os.Exit(1)
	}
}

func parseFlags() config {
	var cfg config
	flag.StringVar(&cfg.dir, "dir", ".", "directory to scan for input files")
	flag.StringVar(&cfg.out, "out", "converted", "output directory")
	flag.StringVar(&cfg.ext, "ext", "mkv", "input file extension (without dot)")
	flag.StringVar(&cfg.bitrate, "bitrate", "1536k", "E-AC3 target bitrate")
	flag.IntVar(&cfg.jobs, "jobs", 1, "number of files to convert in parallel")
	flag.StringVar(&cfg.ffmpeg, "ffmpeg", "ffmpeg", "path to ffmpeg binary")
	flag.StringVar(&cfg.ffprobe, "ffprobe", "ffprobe", "path to ffprobe binary")
	flag.StringVar(&cfg.logPath, "log", "convert_log.txt", "log file path")
	flag.BoolVar(&cfg.noColor, "no-color", false, "disable ANSI colors")
	flag.BoolVar(&cfg.noNotify, "no-notify", false, "disable desktop notifications")
	flag.BoolVar(&cfg.dryRun, "dry-run", false, "analyze only, don't convert")
	flag.Parse()
	if cfg.jobs < 1 {
		cfg.jobs = 1
	}
	return cfg
}

func run(cfg config, r *Renderer, notifier Notifier) error {
	// Resolve binaries early with a clear error if missing.
	if err := ensureBinaries(cfg.ffmpeg, cfg.ffprobe); err != nil {
		return err
	}

	logger, err := NewLogger(cfg.logPath)
	if err != nil {
		return err
	}
	defer logger.Close()
	logger.Line("DTS \u2192 E-AC3 batch conversion started")

	// Ctrl-C cancels in-flight ffmpeg and removes partial outputs.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pattern := filepath.Join(cfg.dir, "*."+cfg.ext)
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	sort.Strings(files)

	printBanner(r, cfg, len(files))
	if len(files) == 0 {
		r.Plain(r.pal.yellow("No *." + cfg.ext + " files found in " + cfg.dir))
		return nil
	}

	if err := os.MkdirAll(cfg.out, 0o755); err != nil {
		return err
	}

	// ---- analysis phase --------------------------------------------------
	jobs, skipExists, skipNoDTS := analyze(ctx, cfg, r, logger, files)

	if cfg.dryRun {
		r.Plain(r.pal.dim("dry-run: nothing was converted"))
	}

	// ---- conversion phase ------------------------------------------------
	var results []result
	if !cfg.dryRun && len(jobs) > 0 {
		results = convertAll(ctx, cfg, r, logger, jobs)
	}

	// ---- summary ---------------------------------------------------------
	sum := summarize(results, len(skipExists), len(skipNoDTS))
	r.Plain(renderSummary(r.pal, sum, jobs))
	logger.Line(fmt.Sprintf("Finished: %d converted, %d failed, %d skipped",
		sum.ok, sum.failed, sum.skippedExists+sum.skippedNoDTS))

	notifier.Send(notificationFor(sum))
	return nil
}

// analyze probes every file and partitions them into work / skip buckets.
func analyze(ctx context.Context, cfg config, r *Renderer, logger *Logger, files []string) (jobs []Job, skipExists, skipNoDTS []string) {
	for i, f := range files {
		base := filepath.Base(f)
		name := strings.TrimSuffix(base, filepath.Ext(base))
		outPath := filepath.Join(cfg.out, name+".mkv")

		r.SetLive([]string{fmt.Sprintf("  %s analyzing %s %s",
			r.pal.cyan(spinnerFrame(time.Now())), base,
			r.pal.dim(fmt.Sprintf("(%d/%d)", i+1, len(files))))})

		if fileExists(outPath) {
			skipExists = append(skipExists, base)
			r.Log("  " + r.pal.yellow("⤿ skip ") + base + r.pal.dim(" — output exists"))
			logger.Line("skip (exists): " + base)
			continue
		}

		pr, err := probe(ctx, cfg.ffprobe, f)
		if err != nil {
			r.Log("  " + r.pal.red("✘ probe ") + base + r.pal.dim(" — "+err.Error()))
			logger.Line("probe error: " + base + ": " + err.Error())
			continue
		}
		if !pr.HasDTS {
			skipNoDTS = append(skipNoDTS, base)
			r.Log("  " + r.pal.dim("∅ skip  "+base+" — no DTS ("+codecList(pr.Streams)+")"))
			logger.Line("skip (no dts): " + base)
			continue
		}

		jobs = append(jobs, Job{
			Path: f, Name: name, OutPath: outPath,
			Audio: pr.Streams, Duration: pr.Duration,
		})
		r.Log("  " + r.pal.green("✓ queue ") + base + r.pal.dim(" — "+decisionString(r.pal, pr.Streams)))
		logger.Line("queued: " + base + " [" + decisionStringPlain(pr.Streams) + "]")
	}
	r.SetLive(nil)
	return jobs, skipExists, skipNoDTS
}

// convertAll runs the jobs through a worker pool and renders a live dashboard.
func convertAll(ctx context.Context, cfg config, r *Renderer, logger *Logger, jobs []Job) []result {
	r.Plain(r.pal.bold(fmt.Sprintf("\nConverting %d file(s) with %d worker(s)\n",
		len(jobs), cfg.jobs)))

	dash := newDashboard(cfg.jobs)
	jobCh := make(chan Job)
	resCh := make(chan result, len(jobs))

	// Feed jobs.
	go func() {
		defer close(jobCh)
		for _, j := range jobs {
			select {
			case jobCh <- j:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Workers.
	var wg sync.WaitGroup
	for w := 0; w < cfg.jobs; w++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			for j := range jobCh {
				start := time.Now()
				dash.begin(slot, j.Name)
				err := convert(ctx, cfg.ffmpeg, cfg.bitrate, j,
					func(frac float64) { dash.progress(slot, frac) })
				took := time.Since(start)
				dash.end(slot)

				if err != nil {
					removeFile(j.OutPath) // drop partial output, like the bat
					r.Log("  " + r.pal.red("✘ fail  ") + j.Name + r.pal.dim(" — "+firstLine(err.Error())))
					logger.Line("ERROR: " + j.Name + ": " + err.Error())
				} else {
					r.Log("  " + r.pal.green("✔ done  ") + j.Name +
						r.pal.dim(" — "+fmtDur(took)))
					logger.Line("done: " + j.Name + " in " + fmtDur(took))
				}
				resCh <- result{name: j.Name, err: err, took: took}
			}
		}(w)
	}

	// Repaint the dashboard until all workers finish.
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				r.SetLive(dash.render(r.pal))
			case <-done:
				r.SetLive(nil)
				return
			}
		}
	}()

	wg.Wait()
	close(done)
	close(resCh)

	var results []result
	for res := range resCh {
		results = append(results, res)
	}
	return results
}

// ---- dashboard ---------------------------------------------------------

type slotState struct {
	active bool
	name   string
	frac   float64
	start  time.Time
}

type dashboard struct {
	mu    sync.Mutex
	slots []slotState
}

func newDashboard(n int) *dashboard {
	return &dashboard{slots: make([]slotState, n)}
}

func (d *dashboard) begin(i int, name string) {
	d.mu.Lock()
	d.slots[i] = slotState{active: true, name: name, start: time.Now()}
	d.mu.Unlock()
}

func (d *dashboard) progress(i int, frac float64) {
	d.mu.Lock()
	d.slots[i].frac = frac
	d.mu.Unlock()
}

func (d *dashboard) end(i int) {
	d.mu.Lock()
	d.slots[i].active = false
	d.mu.Unlock()
}

func (d *dashboard) render(p palette) []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	var lines []string
	for _, s := range d.slots {
		if !s.active {
			continue
		}
		bar := progressBar(s.frac, 24)
		pct := fmt.Sprintf("%3.0f%%", s.frac*100)
		lines = append(lines, fmt.Sprintf("  %s %s %s %s %s",
			p.cyan(spinnerFrame(now)),
			p.bold(pct),
			p.magenta(bar),
			truncate(s.name, 40),
			p.dim(fmtDur(now.Sub(s.start))),
		))
	}
	return lines
}
