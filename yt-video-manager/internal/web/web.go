package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// playable media extensions we expose in the watch UI
var mediaExt = map[string]bool{
	".mp4": true, ".mkv": true, ".webm": true, ".mov": true,
	".m4a": true, ".mp3": true, ".opus": true, ".aac": true,
}

type fileInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// Serve starts the watch UI on addr (e.g. ":8080"), streaming files from dir.
// It blocks, so run it in a goroutine.
func Serve(addr, dir string) error {
	mux := http.NewServeMux()

	// stream the actual media files straight off disk (no ads, no network)
	mux.Handle("/media/", http.StripPrefix("/media/", http.FileServer(http.Dir(dir))))

	// JSON list of downloaded media
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var files []fileInfo
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if !mediaExt[strings.ToLower(filepath.Ext(e.Name()))] {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			files = append(files, fileInfo{Name: e.Name(), Size: info.Size()})
		}
		sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(files)
	})

	// the page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(indexHTML))
	})

	return http.ListenAndServe(addr, mux)
}

const indexHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>ytm — watch</title>
<style>
  :root { color-scheme: dark; }
  body { margin:0; font-family: system-ui, sans-serif; background:#0f1115; color:#e6e6e6; }
  header { padding:14px 18px; background:#171a21; border-bottom:1px solid #232734; }
  header h1 { margin:0; font-size:16px; }
  header span { color:#8b93a7; font-size:12px; }
  .wrap { display:flex; height: calc(100vh - 58px); }
  .list { width:320px; border-right:1px solid #232734; overflow:auto; }
  .item { padding:10px 14px; cursor:pointer; border-bottom:1px solid #1b1f29; font-size:13px; }
  .item:hover { background:#1b1f29; }
  .item.active { background:#1f3a2e; }
  .item .sz { color:#8b93a7; font-size:11px; margin-top:2px; }
  .player { flex:1; display:flex; align-items:center; justify-content:center; padding:18px; }
  video { width:100%; max-height:100%; background:#000; border-radius:8px; }
  .empty { color:#8b93a7; padding:18px; font-size:13px; }
</style>
</head>
<body>
<header>
  <h1>ytm — watch</h1>
  <span>local downloads &middot; no ads, no tracking</span>
</header>
<div class="wrap">
  <div class="list" id="list"><div class="empty">loading...</div></div>
  <div class="player"><video id="player" controls></video></div>
</div>
<script>
  function fmtSize(n){
    var u = ["B","KB","MB","GB"], i = 0;
    while (n >= 1024 && i < u.length - 1) { n /= 1024; i++; }
    return n.toFixed(1) + " " + u[i];
  }
  function load(){
    fetch("/files").then(function(r){ return r.json(); }).then(function(files){
      var list = document.getElementById("list");
      list.innerHTML = "";
      if (!files || files.length === 0) {
        list.innerHTML = '<div class="empty">no downloads yet</div>';
        return;
      }
      files.forEach(function(f){
        var d = document.createElement("div");
        d.className = "item";
        var name = document.createElement("div");
        name.textContent = f.name;
        var sz = document.createElement("div");
        sz.className = "sz";
        sz.textContent = fmtSize(f.size);
        d.appendChild(name);
        d.appendChild(sz);
        d.onclick = function(){
          var items = document.querySelectorAll(".item");
          for (var k = 0; k < items.length; k++) items[k].classList.remove("active");
          d.classList.add("active");
          var p = document.getElementById("player");
          p.src = "/media/" + encodeURIComponent(f.name);
          p.play();
        };
        list.appendChild(d);
      });
    }).catch(function(){
      document.getElementById("list").innerHTML = '<div class="empty">error loading files</div>';
    });
  }
  load();
  setInterval(load, 4000);
</script>
</body>
</html>`