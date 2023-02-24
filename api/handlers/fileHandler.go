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
	"time"

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

type (
	FileGetStatResp = utils.APIResponse[FileGetStatRespData]

	FileGetStatRespData struct {
		Stat StatInfo `json:"stat"`
	}

	StatInfo struct {
		Name    string    `json:"name"`
		IsDir   bool      `json:"isDir"`
		Size    int64     `json:"size"`
		ModTime time.Time `json:"modTime"`
	}
)

func (h FileHandler) Get(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimSpace(chi.URLParam(r, "path"))
	if rawPath == "" {
		utils.WriteAPIErr(w, http.StatusBadRequest, "path is required")
		return
	}

	filePath, spaceName, err := utils.SpacePathToNormalPath(rawPath, h.Spaces)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusBadRequest, err.Error())
		return
	}

	uInfo := r.Context().Value(utils.UserContextKey).(*utils.UserInfo)
	if _, ok := uInfo.Spaces[spaceName]; !ok {
		utils.WriteAPIErr(w, http.StatusUnauthorized, "unauthrized to access space")
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

	filePath, spaceName, err := utils.SpacePathToNormalPath(rawPath, h.Spaces)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusBadRequest, err.Error())
		return
	}

	uInfo := r.Context().Value(utils.UserContextKey).(*utils.UserInfo)
	if _, ok := uInfo.Spaces[spaceName]; !ok {
		utils.WriteAPIErr(w, http.StatusUnauthorized, "unauthrized to access space")
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

	uInfo := r.Context().Value(utils.UserContextKey).(*utils.UserInfo)
	if _, ok := uInfo.Spaces[spaceName]; !ok {
		utils.WriteAPIErr(w, http.StatusUnauthorized, "unauthrized to access space")
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

// TODO stating works for dirs too... But the functionality is the same...
// Should we create a new url like /paths/stat only for stats or accept this thing?
func (h FileHandler) Stat(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimSpace(chi.URLParam(r, "path"))
	if rawPath == "" {
		utils.WriteAPIErr(w, http.StatusBadRequest, "path is required")
		return
	}

	filePath, spaceName, err := utils.SpacePathToNormalPath(rawPath, h.Spaces)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusBadRequest, err.Error())
		return
	}

	uInfo := r.Context().Value(utils.UserContextKey).(*utils.UserInfo)
	if _, ok := uInfo.Spaces[spaceName]; !ok {
		utils.WriteAPIErr(w, http.StatusUnauthorized, "unauthrized to access space")
		return
	}

	stat, err := os.Stat(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			utils.WriteAPIErr(w, http.StatusNotFound, err.Error())
			return
		}
		utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error happened")
		return
	}

	resp := FileGetStatResp{
		OK: true,
		Data: &FileGetStatRespData{
			Stat: StatInfo{
				Name:    stat.Name(),
				IsDir:   stat.IsDir(),
				ModTime: stat.ModTime(),
				Size:    stat.Size(),
			},
		},
	}

	respJson, err := json.Marshal(&resp)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error happened")
		return
	}

	w.Write(respJson)
}
