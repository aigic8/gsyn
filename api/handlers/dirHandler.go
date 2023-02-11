package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/aigic8/gosyn/api/handlers/utils"
	"github.com/go-chi/chi/v5"
)

type DirHandler struct {
	Spaces map[string]string
}

type (
	dirGetListRespData struct {
		Children []dirChild `json:"children"`
	}

	dirChild struct {
		Name  string `json:"name"`
		IsDir bool   `json:"isDir"`
	}
)

type (
	dirGetTreeRespData struct {
		Tree utils.Tree `json:"tree"`
	}
)

func (h DirHandler) GetList(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimSpace(chi.URLParam(r, "path"))
	if rawPath == "" {
		utils.WriteAPIErr(w, http.StatusBadRequest, "path is required")
		return
	}

	dirPath, err := utils.SpacePathToNormalPath(rawPath, h.Spaces)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusBadRequest, err.Error())
		return
	}

	stat, err := os.Stat(dirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			utils.WriteAPIErr(w, http.StatusNotFound, fmt.Sprintf("path '%s' does not exist", dirPath))
		} else {
			utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	if !stat.IsDir() {
		utils.WriteAPIErr(w, http.StatusBadRequest, fmt.Sprintf("path '%s' is not a directory", dirPath))
		return
	}

	rawChildren, err := os.ReadDir(dirPath)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	children := make([]dirChild, 0, len(rawChildren))
	for _, child := range rawChildren {
		children = append(children, dirChild{Name: child.Name(), IsDir: child.IsDir()})
	}

	res := utils.APIResponse[dirGetListRespData]{
		OK:   true,
		Data: &dirGetListRespData{Children: children},
	}

	resBytes, err := json.Marshal(&res)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Write(resBytes)
}

// FIXME use something like maxDepth. Since tree can grow without bounds
func (h DirHandler) GetTree(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimSpace(chi.URLParam(r, "path"))
	if rawPath == "" {
		utils.WriteAPIErr(w, http.StatusBadRequest, "path is required")
		return
	}

	dirPath, err := utils.SpacePathToNormalPath(rawPath, h.Spaces)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusBadRequest, err.Error())
		return
	}

	stat, err := os.Stat(dirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			utils.WriteAPIErr(w, http.StatusNotFound, fmt.Sprintf("path '%s' does not exist", dirPath))
		} else {
			utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	if !stat.IsDir() {
		utils.WriteAPIErr(w, http.StatusBadRequest, fmt.Sprintf("path '%s' is not a directory", dirPath))
		return
	}

	// FIXME show the path based space, maybe server does not want to reveal the full path
	base, dirName := path.Split(dirPath)
	t := utils.Tree{
		dirName: utils.TreeItem{
			Path:     dirPath,
			IsDir:    true,
			Children: map[string]utils.TreeItem{},
		},
	}

	utils.FillTree(base, t)

	res := utils.APIResponse[dirGetTreeRespData]{
		OK:   true,
		Data: &dirGetTreeRespData{Tree: t},
	}

	resBytes, err := json.Marshal(&res)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Write(resBytes)
}
