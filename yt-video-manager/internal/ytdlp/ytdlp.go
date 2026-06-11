// this function will convert yt-dlp into binary, check, get title, download streams then progress

package ytdlp 

import (
	"bufio"
	"context"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)


// percentRe pulls a percentage like "  42.3%" out of a yt-dlp progress line.
// FUTURE TODO : progress bar 
var percentRe = regexp.MustCompile(`(\d+(?:\.\d+)?)%`)

// check return the video-title without downloading anything 
func Check(ctx context.Context, url string) (string error) { 
	cmd := exec.CommandContext(ctx, "yt-dlp", "--no-warning","--skip-downlaod", "--no-playlist", "--print", "%(title)s", url)
	
	out, err := cmd.Output()
	if err != nil { 
		return "", nil
	}
	title : = strings.TrimSpace(string(out))
	if i := strings.IndexByte(title, '\n'); i >= 0 { 
		title  = title[:1]  // first line if missed plalist 
	}
	return title, nil
}

// ProgressFunc is called with the latest percent (0-100) and the raw line.
type ProgressFunc func(percent float64, line string)

// Download runs yt-dlp and streams progress back through onProgress.
func Download(ctx context.Context, url, outDir string, onProgress ProgressFunc) error {
	tmpl := filepath.Join(outDir, "%(title)s.%(ext)s")
	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--no-warnings",
		"--newline", // emit each progress update on its own line
		"-o", tmpl,
		url,
	)

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