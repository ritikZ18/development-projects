package main 


import ( 
		"bufio"
	"fmt"
	"os"
	"strings"

	"ytm/internal/config"
	"ytm/internal/ui"
)

func main(){ 
		if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := runInit(); err != nil {
			fmt.Fprintln(os.Stderr, "init error:", err)
			os.Exit(1)
		}
		return
	}

	if err := ui.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// runInit asks for a download folder once and writes the config file.
func runInit() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	fmt.Printf("Download folder [%s]: ", cfg.DownloadDir)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line != "" {
		cfg.DownloadDir = line
	}

	if err := os.MkdirAll(cfg.DownloadDir, 0o755); err != nil {
		return err
	}
	if err := cfg.Save(); err != nil {
		return err
	}

	fmt.Println("Saved. Download folder:", cfg.DownloadDir)
	return nil
}