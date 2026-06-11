# ytm — a yt-dlp queue manager (Go + Bubble Tea)

An interactive terminal tool that wraps **yt-dlp**. Paste single or bulk links,
check titles, pick a folder, and download — all from a live slash-command screen.
``

How the live updates work: each download runs in its own goroutine and pushes
`progressMsg` values onto a channel. A Bubble Tea command (`listen`) drains the
channel one message per update cycle and re-arms itself, so the screen
re-renders continuously as percentages climb — no polling.

yt-dlp is invoked as a subprocess. `Check` uses `--print "%(title)s"` to grab a
title with no download; `Download` uses `--newline` and the percentages are
parsed off stdout with a regex.

## Commands

| command            | what it does                                   |
| ------------------ | ---------------------------------------------- |
| paste a link ⏎     | adds it to the queue (same as `/add`)          |
| `/add <url> ...`   | add one or more links (space-separated)        |
| `/check [id ...]`  | fetch titles for pending links (or given ids)  |
| `/list`            | show how many items are queued                 |
| `/folder [path]`   | set the download folder, or show it            |
| `/download`        | download everything not already done           |
| `/remove <id>`     | drop an item                                   |
| `/clear`           | empty the queue                                |
| `/help`            | list commands                                  |
| `/quit`            | exit (or ctrl+c)                               |

Type `/` and the palette filters as you type.

