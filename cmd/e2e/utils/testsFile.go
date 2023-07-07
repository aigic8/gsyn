package utils

import (
	"errors"
	"io"
	"os"

	"github.com/pelletier/go-toml/v2"
)

type (
	TestFileData struct {
		Cert    TestFileCertData     `toml:"cert"`
		Users   []TestFileUserData   `toml:"users"`
		Servers []TestFileServerData `toml:"servers"`
		Spaces  map[string]string    `toml:"spaces"`
		Tests   []TestFileTestData   `toml:"tests"`
	}

	TestFileCertData struct {
		Key string `toml:"key"`
		Pem string `toml:"pem"`
		Der string `toml:"der"`
	}

	TestFileUserData struct {
		Name    string   `toml:"name"`
		UUID    string   `toml:"UUID"`
		Servers []string `toml:"servers"`
	}

	TestFileServerData struct {
		Name    string               `toml:"name"`
		Address string               `toml:"address"`
		Users   []TestFileServerUser `toml:"users"`
	}

	TestFileServerUser struct {
		Name   string   `toml:"name"`
		Spaces []string `toml:"spaces"`
	}

	TestFileTestData struct {
		User           string   `toml:"user"`
		Server         string   `toml:"server"`
		Name           string   `toml:"name"`
		MakeFiles      []string `toml:"makeFiles"`
		Commands       []string `toml:"commands"`
		ExpectFiles    []string `toml:"expectFiles"`
		ExpectDirs     []string `toml:"expectDirs"`
		ExpectError    bool     `toml:"expectError"`
		MakeSpacesDirs bool     `toml:"makeSpacesDirs"`
	}
)

// TODO: validate tests file
func ParseTestFile(testFilePath string) (*TestFileData, error) {
	file, err := os.Open(testFilePath)
	if err != nil {
		return nil, err
	}

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var data TestFileData
	if err = toml.Unmarshal(fileBytes, &data); err != nil {
		return nil, err
	}

	return &data, nil
}

func MakeUsersAndServersMap(usersArr []TestFileUserData, serversArr []TestFileServerData) (map[string]TestFileUserData, map[string]TestFileServerData, error) {
	usersMap := map[string]TestFileUserData{}
	for _, user := range usersArr {
		if _, userExists := usersMap[user.Name]; userExists {
			return nil, nil, errors.New("error: mutiple users with the same name " + user.Name)
		}
		usersMap[user.Name] = user
	}

	serversMap := map[string]TestFileServerData{}
	for _, server := range serversArr {
		if _, serverExists := serversMap[server.Name]; serverExists {
			return nil, nil, errors.New("error: mutiple servers with the same name " + server.Name)
		}
		serversMap[server.Name] = server
	}

	return usersMap, serversMap, nil
}
