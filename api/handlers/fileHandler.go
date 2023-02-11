package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/aigic8/gosyn/api/handlers/utils"
	"github.com/go-chi/chi/v5"
)

type FileHandler struct {
	Spaces map[string]string
}

func (h FileHandler) Get(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimSpace(chi.URLParam(r, "path"))
	if rawPath == "" {
		utils.WriteAPIErr(w, http.StatusBadRequest, "path is required")
		return
	}

	filePath, err := utils.SpacePathToNormalPath(rawPath, h.Spaces)
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

	filePath, err := utils.SpacePathToNormalPath(rawPath, h.Spaces)
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
