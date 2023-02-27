package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/aigic8/gosyn/api/handlers/utils"
	"github.com/aigic8/gosyn/api/pb"
	"github.com/go-chi/chi/v5"
	"google.golang.org/protobuf/proto"
)

type DirHandler struct {
	Spaces map[string]string
}

func (h DirHandler) GetList(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimSpace(chi.URLParam(r, "path"))
	if rawPath == "" {
		utils.WriteAPIErr(w, http.StatusBadRequest, "path is required")
		return
	}

	dirPath, spaceName, err := utils.SpacePathToNormalPath(rawPath, h.Spaces)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusBadRequest, err.Error())
		return
	}

	uInfo := r.Context().Value(utils.UserContextKey).(*utils.UserInfo)
	if _, ok := uInfo.Spaces[spaceName]; !ok {
		utils.WriteAPIErr(w, http.StatusUnauthorized, "unauthrized to access space")
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

	children := make([]*pb.DirChild, 0, len(rawChildren))
	for _, child := range rawChildren {
		children = append(children, &pb.DirChild{Name: child.Name(), IsDir: child.IsDir()})
	}

	res := pb.DirGetListResponse{Children: children}
	resBytes, err := proto.Marshal(&res)
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

	dirPath, spaceName, err := utils.SpacePathToNormalPath(rawPath, h.Spaces)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusBadRequest, err.Error())
		return
	}

	uInfo := r.Context().Value(utils.UserContextKey).(*utils.UserInfo)
	if _, ok := uInfo.Spaces[spaceName]; !ok {
		utils.WriteAPIErr(w, http.StatusUnauthorized, "unauthrized to access space")
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
	t := map[string]*pb.TreeItem{
		dirName: {
			Path:     dirPath,
			IsDir:    true,
			Children: map[string]*pb.TreeItem{},
		},
	}

	utils.FillTree(base, t)

	res := pb.DirGetTreeResponse{Tree: t}
	resBytes, err := proto.Marshal(&res)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Write(resBytes)
}
