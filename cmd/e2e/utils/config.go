package utils

import (
	"fmt"
	"os"
	"path"

	"github.com/aigic8/gosyn/cmd/gsyn/config"
	"github.com/pelletier/go-toml/v2"
)

type ConfigInfo struct {
	Address          string
	ServerConfigPath string
	ClientConfigPath string
}

func (c *ConfigInfo) Clean() error {
	if err := os.Remove(c.ServerConfigPath); err != nil {
		return err
	}
	return os.Remove(c.ClientConfigPath)
}

type ConfigOptions struct {
	Users                        []config.ServerUser
	Servers                      map[string]config.ClientServerItem
	Spaces                       map[string]string
	PrivKeyPem, CertPem, CertDer string
}

func GenerateConfigs(o ConfigOptions) (*ConfigInfo, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	serverPort := 8080
	serverConfig := config.Config{
		Server: &config.ServerConfig{
			Spaces:   o.Spaces,
			Address:  fmt.Sprintf(":%d", serverPort),
			PrivPath: o.PrivKeyPem,
			CertPath: o.CertPem,
			Users:    o.Users,
		},
	}

	serverConfigPath := path.Join(cwd, "server.config.toml")
	if err = generateConfigFile(serverConfigPath, &serverConfig); err != nil {
		return nil, err
	}

	clientConfig := config.Config{
		Client: &config.ClientConfig{
			DefaultTimeout: 10000,
			DefaultWorkers: 10,
			Servers:        o.Servers,
		},
	}
	clientConfigPath := path.Join(cwd, "client.config.toml")
	if err = generateConfigFile(clientConfigPath, &clientConfig); err != nil {
		return nil, err
	}

	return &ConfigInfo{
		Address:          fmt.Sprintf("https://localhost:%d", serverPort),
		ServerConfigPath: serverConfigPath,
		ClientConfigPath: clientConfigPath,
	}, nil
}

func generateConfigFile(configPath string, config *config.Config) error {
	configBytes, err := toml.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, configBytes, 0777)
}
