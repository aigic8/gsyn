package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
)

type (
	Tree = map[string]TreeItem

	TreeItem struct {
		Path     string              `json:"path"`
		IsDir    bool                `json:"isDir"`
		Children map[string]TreeItem `json:"children"`
	}
)

func FillTree(base string, tree map[string]TreeItem) error {
	for key, item := range tree {
		if !item.IsDir {
			continue
		}

		curr := item.Path
		children, err := os.ReadDir(curr)
		if err != nil {
			return err
		}

		for _, child := range children {
			childName := child.Name()
			tree[key].Children[childName] = TreeItem{
				IsDir:    child.IsDir(),
				Path:     path.Join(curr, childName),
				Children: map[string]TreeItem{},
			}
		}

		if err = FillTree(curr, tree[key].Children); err != nil {
			return err
		}
	}

	return nil
}

func SpacePathToNormalPath(rawPath string, spaces map[string]string) (string, error) {
	spaceName, filePath, err := SplitSpaceAndPath(rawPath)
	if err != nil {
		return "", fmt.Errorf("bad path: %v", err)
	}

	spacePath, ok := spaces[spaceName]
	if !ok {
		return "", errors.New("space does not exist")
	}

	return path.Join(spacePath, filePath), nil
}

// Splits space name and path in the space. Only works if the string is trimmed
func SplitSpaceAndPath(rawPath string) (string, string, error) {
	pathParts := strings.SplitN(rawPath, "/", 2)

	if pathParts[0] == "" {
		return "", "", errors.New("space name is empty")
	}

	if len(pathParts) == 1 {
		return pathParts[0], "", nil
	}

	return pathParts[0], pathParts[1], nil
}

type APIResponse[T any] struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Data  *T     `json:"data,omitempty"`
}

func WriteAPIErr(w http.ResponseWriter, status int, error string) error {
	w.WriteHeader(status)
	resp := APIResponse[bool]{
		OK:    false,
		Error: error,
	}
	respData, err := json.Marshal(&resp)
	if err != nil {
		return err
	}

	if _, err = w.Write(respData); err != nil {
		return err
	}

	return nil
}
