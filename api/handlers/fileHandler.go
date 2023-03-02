package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aigic8/gosyn/api/handlers/utils"
	"github.com/aigic8/gosyn/api/pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type FileHandler struct {
	Spaces map[string]string
}

func (h FileHandler) Get(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimSpace(r.URL.Query().Get("path"))
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

	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))

	io.Copy(w, file)
}

func (h FileHandler) PutNew(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimSpace(r.Header.Get("x-file-path"))
	srcName := strings.TrimSpace(r.Header.Get("x-src-name"))
	isForced := r.Header.Get("x-force") == "true"

	if rawPath == "" {
		utils.WriteAPIErr(w, http.StatusBadRequest, "file path is required")
		return
	}

	if srcName == "" {
		utils.WriteAPIErr(w, http.StatusBadRequest, "source name is required")
		return
	}

	if strings.ContainsRune(srcName, '/') {
		utils.WriteAPIErr(w, http.StatusBadRequest, "source name can not contain '/'")
		return
	}

	destPath, spaceName, err := utils.SpacePathToNormalPath(rawPath, h.Spaces)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusBadRequest, err.Error())
		return
	}

	uInfo := r.Context().Value(utils.UserContextKey).(*utils.UserInfo)
	if _, ok := uInfo.Spaces[spaceName]; !ok {
		utils.WriteAPIErr(w, http.StatusUnauthorized, "unauthrized to access space")
		return
	}

	dirMode := false
	wPath := destPath
	fileStat, err := os.Stat(destPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error happened")
			return
		}
	} else {
		if fileStat.IsDir() {
			dirMode = true
			wPath = path.Join(destPath, srcName)
		}
	}

	if !dirMode {
		parentPath := path.Dir(destPath)
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
	}

	// FIXME check for if path is in space space

	wStat, err := os.Stat(wPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error happened")
			return
		}
	} else { // path does exist
		if wStat.IsDir() {
			utils.WriteAPIErr(w, http.StatusBadRequest, fmt.Sprintf("path '%s' is a directory", wPath))
			return
		}
		if !isForced {
			utils.WriteAPIErr(w, http.StatusBadRequest, fmt.Sprintf("path '%s' already exists", wPath))
			return
		}
	}

	file, err := os.Create(wPath)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error happened")
		return
	}
	defer file.Close()

	if _, err = io.Copy(file, r.Body); err != nil {
		utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error happened")
		return
	}

	w.Write([]byte{})
}

func (h FileHandler) Match(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimSpace(r.URL.Query().Get("pattern"))
	if rawPath == "" {
		utils.WriteAPIErr(w, http.StatusBadRequest, "pattern is required")
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

	resp := pb.FileGetMatchResponse{
		Matches: matchedFiles,
	}

	respProto, err := proto.Marshal(&resp)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error happened")
		return
	}

	w.Write(respProto)
}

// TODO stating works for dirs too... But the functionality is the same...
// Should we create a new url like /paths/stat only for stats or accept this thing?
func (h FileHandler) Stat(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimSpace(r.URL.Query().Get("path"))
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

	resp := pb.GetStatResponse{
		Stat: &pb.StatInfo{
			Name:    stat.Name(),
			IsDir:   stat.IsDir(),
			ModTime: timestamppb.New(stat.ModTime()),
			Size:    stat.Size(),
		},
	}

	respProto, err := proto.Marshal(&resp)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error happened")
		return
	}

	w.Write(respProto)
}
