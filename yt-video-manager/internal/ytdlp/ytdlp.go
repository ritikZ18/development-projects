package ytdlp

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// percentRe pulls a percentage like "  42.3%" out of a yt-dlp progress line.
var percentRe = regexp.MustCompile(`(\d+(?:\.\d+)?)%`)

// Check returns the video's title without downloading anything.
func Check(ctx context.Context, url string) (string, error) {
	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--no-warnings",
		"--skip-download",
		"--no-playlist",
		"--print", "%(title)s",
		url,
	)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	title := strings.TrimSpace(string(out))
	if i := strings.IndexByte(title, '\n'); i >= 0 {
		title = title[:i] // first line if a playlist slipped through
	}
	return title, nil
}

// ProgressFunc is called with the latest percent (0-100) and the raw line.
type ProgressFunc func(percent float64, line string)

// Download runs yt-dlp for the given quality and streams progress.
// quality is one of: best, 1080, 720, 480, 360, audio.
// Video is written as browser-friendly mp4 (h264/aac) so it plays in any browser.
func Download(ctx context.Context, url, outDir, quality string, onProgress ProgressFunc) error {
	tmpl := filepath.Join(outDir, "%(title)s.%(ext)s")

	args := []string{"--no-warnings", "--newline", "-o", tmpl}
	switch quality {
	case "audio":
		args = append(args, "-x", "--audio-format", "mp3")
	case "1080", "720", "480", "360":
		args = append(args,
			"-f", fmt.Sprintf("bv*[height<=%s]+ba/b[height<=%s]/b", quality, quality),
			"-S", "vcodec:h264,acodec:aac",
			"--merge-output-format", "mp4",
		)
	default: // best
		args = append(args,
			"-f", "bv*+ba/b",
			"-S", "vcodec:h264,acodec:aac",
			"--merge-output-format", "mp4",
		)
	}
	args = append(args, url)

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// Drain stderr so the process never blocks on a full pipe.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s := bufio.NewScanner(stderr)
		for s.Scan() {
		}
	}()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if onProgress == nil {
			continue
		}
		if m := percentRe.FindStringSubmatch(line); m != nil {
			if p, perr := strconv.ParseFloat(m[1], 64); perr == nil {
				onProgress(p, line)
			}
		}
	}

	wg.Wait()
	return cmd.Wait()
}