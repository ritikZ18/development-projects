package ui

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ytm/internal/config"
	"ytm/internal/queue"
	"ytm/internal/web"
	"ytm/internal/ytdlp"
)

const watchAddr = ":8080"

// ----- styles -----

var (
	titleStyle = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("63")).Padding(0, 1)
	dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	okStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	cmdStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
)

// ----- slash commands shown in the palette -----

var commands = []struct{ name, desc string }{
	{"/add", "add one or more links to the queue"},
	{"/check", "fetch titles for pending links"},
	{"/quality", "set quality: best|1080|720|480|audio"},
	{"/download", "download everything in the queue"},
	{"/files", "list downloaded files on disk"},
	{"/list", "show how many items are queued"},
	{"/folder", "set or show the download folder"},
	{"/remove", "remove an item by id"},
	{"/clear", "empty the queue"},
	{"/help", "list the commands"},
	{"/quit", "exit"},
}

// ----- messages from background work -----

type checkResultMsg struct {
	id    int
	title string
	err   error
}

type progressMsg struct {
	id      int
	percent float64
}

type downloadDoneMsg struct {
	id  int
	err error
}

// ----- model -----

type model struct {
	cfg    config.Config
	q      *queue.Queue
	input  textinput.Model
	log    []string
	events chan tea.Msg // background goroutines push progress/done here
}

// Run sets up config, starts the watch server, and runs the TUI.
func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.DownloadDir, 0o755); err != nil {
		return err
	}

	// watch UI in the background; serves the folder we start with
	go func() { _ = web.Serve(watchAddr, cfg.DownloadDir) }()

	p := tea.NewProgram(newModel(cfg), tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func newModel(cfg config.Config) model {
	ti := textinput.New()
	ti.Placeholder = "paste a link, or type / for commands"
	ti.Prompt = "› "
	ti.CharLimit = 0
	ti.Width = 60
	ti.Focus()

	return model{
		cfg:    cfg,
		q:      queue.New(),
		input:  ti,
		events: make(chan tea.Msg, 256),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, listen(m.events))
}

// listen waits for one background message, then is re-armed by Update.
func listen(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

func checkCmd(id int, url string) tea.Cmd {
	return func() tea.Msg {
		title, err := ytdlp.Check(context.Background(), url)
		return checkResultMsg{id: id, title: title, err: err}
	}
}

func (m model) startDownload(it *queue.Item) {
	events := m.events
	id, url, dir, quality := it.ID, it.URL, m.cfg.DownloadDir, m.cfg.Format
	go func() {
		err := ytdlp.Download(context.Background(), url, dir, quality, func(p float64, _ string) {
			events <- progressMsg{id: id, percent: p}
		})
		events <- downloadDoneMsg{id: id, err: err}
	}()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		if msg.Width > 6 {
			m.input.Width = msg.Width - 4
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			var cmd tea.Cmd
			m, cmd = m.handleSubmit()
			m.input.SetValue("")
			return m, cmd
		}

	case checkResultMsg:
		if it := m.q.Get(msg.id); it != nil {
			if msg.err != nil {
				it.Status = queue.Failed
				it.Err = shortErr(msg.err)
				m.log = append(m.log, fmt.Sprintf("#%d check failed: %s", msg.id, it.Err))
			} else {
				it.Title = msg.title
				it.Status = queue.Checked
			}
		}
		return m, nil

	case progressMsg:
		if it := m.q.Get(msg.id); it != nil {
			it.Status = queue.Downloading
			it.Progress = msg.percent
		}
		return m, listen(m.events)

	case downloadDoneMsg:
		if it := m.q.Get(msg.id); it != nil {
			if msg.err != nil {
				it.Status = queue.Failed
				it.Err = shortErr(msg.err)
				m.log = append(m.log, fmt.Sprintf("#%d failed: %s", msg.id, it.Err))
			} else {
				it.Status = queue.Done
				it.Progress = 100
				m.log = append(m.log, fmt.Sprintf("#%d done", msg.id))
			}
		}
		return m, listen(m.events)
	}

	// Everything else feeds the text input (typing, cursor blink, etc.)
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// handleSubmit parses the input line on Enter and returns any work to run.
func (m model) handleSubmit() (model, tea.Cmd) {
	raw := strings.TrimSpace(m.input.Value())
	if raw == "" {
		return m, nil
	}

	// Bare text (no leading slash) is treated as link(s) to add.
	if !strings.HasPrefix(raw, "/") {
		return m.addLinks(strings.Fields(raw))
	}

	fields := strings.Fields(raw)
	args := fields[1:]
	switch fields[0] {

	case "/add":
		return m.addLinks(args)

	case "/check":
		return m.checkItems(args)

	case "/quality":
		if len(args) == 0 {
			m.log = append(m.log, "quality: "+m.cfg.Format+"  (best|1080|720|480|audio)")
			return m, nil
		}
		switch args[0] {
		case "best", "1080", "720", "480", "360", "audio":
			m.cfg.Format = args[0]
			_ = m.cfg.Save()
			m.log = append(m.log, "quality set to "+args[0])
		default:
			m.log = append(m.log, "unknown quality (best|1080|720|480|audio)")
		}
		return m, nil

	case "/download":
		return m.downloadAll()

	case "/files":
		return m.listFiles()

	case "/list":
		m.log = append(m.log, fmt.Sprintf("%d item(s) in queue", m.q.Len()))
		return m, nil

	case "/folder":
		rest := strings.TrimSpace(strings.TrimPrefix(raw, "/folder"))
		if rest == "" {
			m.log = append(m.log, "folder: "+m.cfg.DownloadDir)
			return m, nil
		}
		m.cfg.DownloadDir = rest
		_ = os.MkdirAll(rest, 0o755)
		if err := m.cfg.Save(); err != nil {
			m.log = append(m.log, "could not save config: "+shortErr(err))
		} else {
			m.log = append(m.log, "folder set to "+rest+" (restart to update the watch page)")
		}
		return m, nil

	case "/remove":
		if len(args) == 0 {
			m.log = append(m.log, "usage: /remove <id>")
			return m, nil
		}
		id, err := strconv.Atoi(args[0])
		if err != nil {
			m.log = append(m.log, "invalid id")
			return m, nil
		}
		if m.q.Remove(id) {
			m.log = append(m.log, fmt.Sprintf("removed #%d", id))
		} else {
			m.log = append(m.log, "no such id")
		}
		return m, nil

	case "/clear":
		m.q.Clear()
		m.log = append(m.log, "queue cleared")
		return m, nil

	case "/help":
		m.log = append(m.log, "commands: /add /check /quality /download /files /list /folder /remove /clear /quit")
		return m, nil

	case "/quit", "/exit":
		return m, tea.Quit

	default:
		m.log = append(m.log, "unknown command "+fields[0]+" (try /help)")
		return m, nil
	}
}

func (m model) addLinks(urls []string) (model, tea.Cmd) {
	if len(urls) == 0 {
		m.log = append(m.log, "usage: /add <url> [url2 ...]")
		return m, nil
	}
	n := 0
	for _, u := range urls {
		if u == "" {
			continue
		}
		m.q.Add(u)
		n++
	}
	m.log = append(m.log, fmt.Sprintf("added %d link(s)", n))
	return m, nil
}

func (m model) checkItems(args []string) (model, tea.Cmd) {
	var targets []*queue.Item
	if len(args) > 0 {
		for _, a := range args {
			if id, err := strconv.Atoi(a); err == nil {
				if it := m.q.Get(id); it != nil {
					targets = append(targets, it)
				}
			}
		}
	} else {
		for _, it := range m.q.Items() {
			if it.Status == queue.Pending {
				targets = append(targets, it)
			}
		}
	}

	var cmds []tea.Cmd
	for _, it := range targets {
		it.Status = queue.Checking
		cmds = append(cmds, checkCmd(it.ID, it.URL))
	}
	if len(cmds) == 0 {
		m.log = append(m.log, "nothing to check")
		return m, nil
	}
	m.log = append(m.log, fmt.Sprintf("checking %d item(s)...", len(cmds)))
	return m, tea.Batch(cmds...)
}

func (m model) downloadAll() (model, tea.Cmd) {
	n := 0
	for _, it := range m.q.Items() {
		if it.Status == queue.Done || it.Status == queue.Downloading {
			continue
		}
		it.Status = queue.Downloading
		it.Progress = 0
		m.startDownload(it)
		n++
	}
	if n == 0 {
		m.log = append(m.log, "nothing to download")
		return m, nil
	}
	m.log = append(m.log, fmt.Sprintf("downloading %d item(s) at %s...", n, m.cfg.Format))
	return m, nil
}

func (m model) listFiles() (model, tea.Cmd) {
	entries, err := os.ReadDir(m.cfg.DownloadDir)
	if err != nil {
		m.log = append(m.log, "could not read folder: "+shortErr(err))
		return m, nil
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		size := int64(0)
		if info, ierr := e.Info(); ierr == nil {
			size = info.Size()
		}
		m.log = append(m.log, fmt.Sprintf("  %s (%s)", truncate(e.Name(), 44), humanSize(size)))
		n++
	}
	if n == 0 {
		m.log = append(m.log, "no files in "+m.cfg.DownloadDir)
	} else {
		m.log = append(m.log, fmt.Sprintf("%d file(s) in %s", n, m.cfg.DownloadDir))
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(" ytm — youtube queue ") + "\n")
	b.WriteString(dimStyle.Render("folder: "+m.cfg.DownloadDir+"   quality: "+m.cfg.Format) + "\n")
	b.WriteString(dimStyle.Render("watch:  http://localhost"+watchAddr) + "\n\n")

	if m.q.Len() == 0 {
		b.WriteString(dimStyle.Render("queue is empty — paste a link and press enter, or type /help") + "\n")
	} else {
		for _, it := range m.q.Items() {
			b.WriteString(renderItem(it) + "\n")
		}
	}
	b.WriteString("\n")

	// Last several log lines.
	start := 0
	if len(m.log) > 10 {
		start = len(m.log) - 10
	}
	for _, l := range m.log[start:] {
		b.WriteString(dimStyle.Render("· "+l) + "\n")
	}

	// Command palette appears while typing a slash command.
	val := m.input.Value()
	if strings.HasPrefix(val, "/") {
		b.WriteString("\n")
		for _, c := range commands {
			if strings.HasPrefix(c.name, val) {
				b.WriteString("  " + cmdStyle.Render(fmt.Sprintf("%-10s", c.name)) +
					dimStyle.Render(c.desc) + "\n")
			}
		}
	}

	b.WriteString("\n" + m.input.View() + "\n")
	b.WriteString(dimStyle.Render("enter: run · ctrl+c: quit"))
	return b.String()
}

// ----- rendering helpers -----

func renderItem(it *queue.Item) string {
	name := it.Title
	if name == "" {
		name = it.URL
	}
	switch it.Status {
	case queue.Downloading:
		return fmt.Sprintf("  #%-2d %s %5.1f%%  %s",
			it.ID, bar(it.Progress, 16), it.Progress, truncate(name, 38))
	case queue.Done:
		return fmt.Sprintf("  #%-2d %s %s", it.ID, okStyle.Render("done "), truncate(name, 50))
	case queue.Failed:
		return fmt.Sprintf("  #%-2d %s %s", it.ID, errStyle.Render("fail "), truncate(name, 50))
	case queue.Checked:
		return fmt.Sprintf("  #%-2d %s %s", it.ID, okStyle.Render("ok   "), truncate(name, 50))
	case queue.Checking:
		return fmt.Sprintf("  #%-2d %s %s", it.ID, dimStyle.Render("...  "), truncate(name, 50))
	default:
		return fmt.Sprintf("  #%-2d %s %s", it.ID, dimStyle.Render("·    "), truncate(name, 50))
	}
}

func bar(p float64, width int) string {
	filled := int(p / 100 * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

func humanSize(n int64) string {
	f := float64(n)
	units := []string{"B", "KB", "MB", "GB"}
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}

func shortErr(err error) string {
	s := strings.ReplaceAll(err.Error(), "\n", " ")
	if len(s) > 70 {
		s = s[:70] + "…"
	}
	return s
}