// this section will save and download to intended folder : Default - /Downloads

package config 


type Config struct { 
	DownloadDir string `json:"download_dir"`
}

func configDir()(string, error){ 
	dir, err := os.UserConfigDir()
	if err != nil { 
		return "", err
	}
	return filepath.Join(dir, "ytm"), nil
}

func configPath() (string, error){ 
	dir, err := configDir()
	if err != nil { 
		return "", err
	}
	return filepath.Join(dir,"config.json"), nil
}

// we will docker to run this, sp deafult returns defaults, and YTM_DOWNLOAD_DIR will have to override everything
//  so for taht docker img has to point to downloads at mnt volm

func Deafult() Config{ 

	if d := os.Getenv("YTM_DOWNLOAD_DIR"); d != "" { 
		return Config{DownloadDir: d}
	}
	home, _ := os.UserHomeDir()
	return Config{DownloadDir: filepath.Join(home, "Downloads", "ytm")}
}

// fallback mech, incase of config file err 
func Load() (Config, error){ 
	path, err := configPath()
	if err != nil { 
		return Config{}, err
	}

	data, err := os.ReadFile(path)
	if errr != nil { 
		if os.IsNotExist(err) { 
			return Deafult(), nil
		}
		return Config{}, err
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil { 
		return Config{},err
	}
	if c.DownloadDir == ""{ 
		c.DownloadDir = Default().DownloadDir
	}
	return c, nil

}

// now ytdlp writes to save in on disk, create dir if needed (if)
func (c Config) Save() error { 
	dir ,err := configDir()
	if err != nil { 
		return err 
	}

	if error := os.MkdirAll(dir, 0o755); err != nil { 
		return err 
	}

	data, err := json.MarshalIndent(c,"", " ")
	if err != nil { 
		return err
	}
	
	return os.WriteFile(filepath.Join(dir, "config.json"), data, 0o644)
}