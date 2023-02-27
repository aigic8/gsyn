package utils

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/aigic8/gosyn/api/pb"
	"google.golang.org/protobuf/proto"
)

type (
	Tree = map[string]TreeItem

	TreeItem struct {
		Path     string              `json:"path"`
		IsDir    bool                `json:"isDir"`
		Children map[string]TreeItem `json:"children"`
	}
)

type RequestContextKey int

const (
	UserContextKey RequestContextKey = iota
)

type UserInfo struct {
	GUID   string
	Spaces map[string]bool
}

// TODO test
func UserAuthMiddleware(users map[string]UserInfo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			headerParts := strings.Split(authHeader, " ")
			if len(headerParts) != 2 || headerParts[0] != "simple" {
				WriteAPIErr(w, http.StatusUnauthorized, "bad authentication")
				return
			}

			user, ok := users[headerParts[1]]
			if !ok {
				WriteAPIErr(w, http.StatusUnauthorized, "bad authentication")
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, &user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func FillTree(base string, tree map[string]*pb.TreeItem) error {
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
			tree[key].Children[childName] = &pb.TreeItem{
				IsDir:    child.IsDir(),
				Path:     path.Join(curr, childName),
				Children: map[string]*pb.TreeItem{},
			}
		}

		if err = FillTree(curr, tree[key].Children); err != nil {
			return err
		}
	}

	return nil
}

func SpacePathToNormalPath(rawPath string, spaces map[string]string) (string, string, error) {
	spaceName, filePath, err := SplitSpaceAndPath(rawPath)
	if err != nil {
		return "", "", fmt.Errorf("bad path: %v", err)
	}

	spacePath, ok := spaces[spaceName]
	if !ok {
		return "", "", errors.New("space does not exist")
	}

	return path.Join(spacePath, filePath), spaceName, nil
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

func WriteAPIErr(w http.ResponseWriter, status int, message string) error {
	bytes, err := proto.Marshal(&pb.ApiError{Message: message})
	if err != nil {
		return err
	}

	w.WriteHeader(status)
	if _, err = w.Write(bytes); err != nil {
		return err
	}

	return nil
}
