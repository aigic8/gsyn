package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/pelletier/go-toml/v2"
)

type (
	Config struct {
		Client *ClientConfig `toml:"client,omitempty"`
		Server *ServerConfig `toml:"server,omitempty"`
	}

	ClientConfig struct {
		Servers        map[string]ClientServerItem `toml:"servers" validate:"required"`
		DefaultTimeout int64                       `toml:"defaultTimeout" validate:"gte=0"`
	}

	ClientServerItem struct {
		GUID         string   `toml:"GUID" validate:"required"`
		Address      string   `toml:"address" validate:"required,url"`
		Certificates []string `toml:"certificates"`
	}

	ServerConfig struct {
		Spaces   map[string]string `toml:"spaces" validate:"required"`
		Users    []ServerUser      `toml:"users"`
		Address  string            `toml:"address" validate:"required"`
		CertPath string            `toml:"certPath"  validate:"required"`
		PrivPath string            `toml:"privPath" validate:"required"`
	}

	ServerUser struct {
		GUID   string   `toml:"GUID" validate:"required,uuid4"`
		Spaces []string `toml:"spaces"`
	}
)

func LoadConfig(configPaths []string) (*Config, error) {
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

	validate := validator.New()
	if err = validate.Struct(&config); err != nil {
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
