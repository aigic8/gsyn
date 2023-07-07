package utils

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

func IsPathDir(path string) error {
	stat, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !stat.IsDir() {
		return errors.New("path is not a dir")
	}
	return nil
}

func IsPathFile(path string) error {
	stat, err := os.Stat(path)
	if err != nil {
		return err
	}
	if stat.IsDir() {
		return errors.New("path is dir")
	}
	return nil
}

type Client struct {
	ExecPath   string
	ConfigPath string
}

func NewClient(execPath string, configPath string) Client {
	return Client{ExecPath: execPath, ConfigPath: configPath}
}

func (c Client) Run(command string) error {
	args := strings.Fields(command)
	args = append(args, "-c", c.ConfigPath)
	cmd := exec.Command(c.ExecPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New(string(output))
	}
	return nil
}
