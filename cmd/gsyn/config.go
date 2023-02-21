package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// FIXME add validation
type Config struct {
	Servers        map[string]string `toml:"servers"`
	DefaultTimeout int64             `toml:"defaultTimeout"`
}

func LoadConfig() (*Config, error) {
	configPaths, err := getConfigPaths()
	if err != nil {
		return nil, err
	}

	configPath := findFirstFile(configPaths)
	if configPath == "" {
		configPathsStr := strings.Join(configPaths, "\n")
		return nil, fmt.Errorf("no configuration was found in: \n%s", configPathsStr)
	}

	configFile, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}

	configBytes, err := io.ReadAll(configFile)
	if err != nil {
		return nil, err
	}

	var config Config
	if err = toml.Unmarshal(configBytes, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func getConfigPaths() ([]string, error) {
	OS := runtime.GOOS

	if OS == "linux" || OS == "darwin" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		return []string{"/etc/gsyn/config.toml", path.Join(homeDir, ".config/gsyn/config.toml")}, nil
	}
	if OS == "windows" {
		// TODO support windows config
		// https://softwareengineering.stackexchange.com/q/160097
		return nil, errors.New("TODO")
	}
	return nil, fmt.Errorf("unsupported os '%s'", OS)
}

func findFirstFile(paths []string) string {
	for _, pathStr := range paths {
		stat, err := os.Stat(pathStr)
		if err != nil {
			continue
		}
		if !stat.IsDir() {
			return pathStr
		}
	}

	return ""
}