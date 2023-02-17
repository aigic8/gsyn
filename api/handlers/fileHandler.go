package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/aigic8/gosyn/api/handlers/utils"
	"github.com/go-chi/chi/v5"
)

type FileHandler struct {
	Spaces map[string]string
}

type (
	FileGetMatchResp = utils.APIResponse[FileGetMatchRespData]

	FileGetMatchRespData struct {
		Matches []string `json:"matches"`
	}
)

func (h FileHandler) Get(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimSpace(chi.URLParam(r, "path"))
	if rawPath == "" {
		utils.WriteAPIErr(w, http.StatusBadRequest, "path is required")
		return
	}

	filePath, _, err := utils.SpacePathToNormalPath(rawPath, h.Spaces)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusBadRequest, err.Error())
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			utils.WriteAPIErr(w, http.StatusNotFound, fmt.Sprintf("path '%s' does not exist", filePath))
		} else {
			utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if stat.IsDir() {
		utils.WriteAPIErr(w, http.StatusBadRequest, fmt.Sprintf("path '%s' is a directory", filePath))
		return
	}

	io.Copy(w, file)
}

func (h FileHandler) PutNew(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimSpace(r.Header.Get("x-file-path"))
	isForced := r.Header.Get("x-force") == "true"
	if rawPath == "" {
		utils.WriteAPIErr(w, http.StatusBadRequest, "file path is required")
		return
	}

	filePath, _, err := utils.SpacePathToNormalPath(rawPath, h.Spaces)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusBadRequest, err.Error())
		return
	}

	parentPath := path.Dir(filePath)
	parentStat, err := os.Stat(parentPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			utils.WriteAPIErr(w, http.StatusBadRequest, fmt.Sprintf("parent dir '%s' does not exist", parentPath))
			return
		}
		utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error happened")
		return
	}

	if !parentStat.IsDir() {
		utils.WriteAPIErr(w, http.StatusBadRequest, fmt.Sprintf("parent dir '%s' is not a directory", parentPath))
		return
	}

	// FIXME check for if path is in space space

	fileStat, err := os.Stat(filePath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error happened")
			return
		}
	} else { // path does exist
		if fileStat.IsDir() {
			utils.WriteAPIErr(w, http.StatusBadRequest, fmt.Sprintf("path '%s' is a directory", filePath))
			return
		}
		if !isForced {
			utils.WriteAPIErr(w, http.StatusBadRequest, fmt.Sprintf("path '%s' already exists", filePath))
			return
		}
	}

	file, err := os.Create(filePath)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error happened")
		return
	}
	defer file.Close()

	if _, err = io.Copy(file, r.Body); err != nil {
		utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error happened")
		return
	}

	res := utils.APIResponse[map[string]bool]{OK: true, Data: &map[string]bool{}}
	resBytes, err := json.Marshal(res)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error happened")
		return
	}

	w.Write(resBytes)
}

// TODO test
func (h FileHandler) Match(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimSpace(chi.URLParam(r, "path"))
	if rawPath == "" {
		utils.WriteAPIErr(w, http.StatusBadRequest, "path is required")
		return
	}

	pattern, spaceName, err := utils.SpacePathToNormalPath(rawPath, h.Spaces)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusBadRequest, err.Error())
		return
	}

	matchedPaths, err := filepath.Glob(pattern)
	if err != nil {
		// as said in https://pkg.go.dev/path/filepath#Glob the only error is for malformed patterns
		utils.WriteAPIErr(w, http.StatusBadRequest, "malformed pattern: "+err.Error())
		return
	}

	matchedFiles := []string{}
	for _, matchedPath := range matchedPaths {
		stat, err := os.Stat(matchedPath)
		// FIXME maybe only not return paths with error instead of returning internal server error?
		if err != nil {
			utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error happened")
			return
		}

		normalPathPrefix := h.Spaces[spaceName]
		if !stat.IsDir() {
			newFile := path.Join(spaceName, strings.TrimPrefix(matchedPath, normalPathPrefix))
			matchedFiles = append(matchedFiles, newFile)
		}
	}

	resp := FileGetMatchResp{
		OK:   true,
		Data: &FileGetMatchRespData{Matches: matchedFiles},
	}

	respJson, err := json.Marshal(&resp)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error happened")
		return
	}

	w.Write(respJson)
}
