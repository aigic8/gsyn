package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type APIResponse[T any] struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Data  *T     `json:"data,omitempty"`
}

type (
	PathInfo struct {
		Name  string
		IsDir bool
	}

	GetPathsListResp = []PathInfo
)

type GetTreeResp struct {
	tree Tree
}

func Router() {
	r := chi.NewRouter()

	r.Use(middleware.AllowContentType("application/json"))
	r.Use(middleware.Logger)
	r.Use(middleware.CleanPath)

	r.Get("/files/{file}", func(w http.ResponseWriter, r *http.Request) {
		filePath := chi.URLParam(r, "path")

		if filePath == "" {
			writeAPIErr(w, http.StatusBadRequest, "file paramter is required")
			return
		}

		file, err := os.Open(filePath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				writeAPIErr(w, http.StatusNotFound, fmt.Sprintf("path '%s' does not exist", filePath))
			} else {
				writeAPIErr(w, http.StatusInternalServerError, "internal server error")
			}
			return
		}
		defer file.Close()

		stat, err := file.Stat()
		if err != nil {
			writeAPIErr(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if stat.IsDir() {
			writeAPIErr(w, http.StatusBadRequest, fmt.Sprintf("path '%s' is a directory", filePath))
			return
		}

		io.Copy(w, file)
	})

	r.Put("/files/new", func(w http.ResponseWriter, r *http.Request) {
		filePath := r.Header.Get("x-file-path")
		isForced := r.Header.Get("Forced") == "true"
		exists := true

		if filePath == "" {
			writeAPIErr(w, http.StatusBadRequest, "file paramter is required")
			return
		}

		stat, err := os.Stat(filePath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				exists = false
			} else {
				writeAPIErr(w, http.StatusInternalServerError, "internal server error")
				return
			}
		}

		if stat.IsDir() {
			writeAPIErr(w, http.StatusBadRequest, fmt.Sprintf("path '%s' is a directory", filePath))
			return
		}

		if !isForced && exists {
			writeAPIErr(w, http.StatusBadRequest, fmt.Sprintf("file '%s' already exists", filePath))
			return
		}

		file, err := os.Create(filePath)
		if err != nil {
			writeAPIErr(w, http.StatusInternalServerError, "internal server error")
			return
		}
		defer file.Close()
		io.Copy(file, r.Body)

		res := APIResponse[map[string]bool]{OK: true, Data: &map[string]bool{}}
		resBytes, err := json.Marshal(&res)
		if err != nil {
			writeAPIErr(w, http.StatusInternalServerError, "internal server error")
			return
		}

		w.Write(resBytes)
	})

	r.Get("/paths/list", func(w http.ResponseWriter, r *http.Request) {
		dirPath := chi.URLParam(r, "path")
		if dirPath == "" {
			writeAPIErr(w, http.StatusBadRequest, "path parameter is required")
			return
		}

		children, err := os.ReadDir(dirPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				writeAPIErr(w, http.StatusNotFound, "path '%s' does not exist")
			} else {
				writeAPIErr(w, http.StatusInternalServerError, "internal server error")
			}
			return
		}

		infos := make([]PathInfo, 0, len(children))
		for _, child := range children {
			infos = append(infos, PathInfo{Name: child.Name(), IsDir: child.IsDir()})
		}

		res := APIResponse[GetPathsListResp]{OK: true, Data: &infos}
		resData, err := json.Marshal(&res)
		if err != nil {
			writeAPIErr(w, http.StatusInternalServerError, "internal server error")
			return
		}

		w.Write(resData)
	})

	r.Get("/paths/tree", func(w http.ResponseWriter, r *http.Request) {
		dirPath := chi.URLParam(r, "path")
		if dirPath == "" {
			writeAPIErr(w, http.StatusBadRequest, "path parameter is required")
			return
		}

		dir, err := os.Stat(dirPath)
		if err != nil {
			writeAPIErr(w, http.StatusBadRequest, fmt.Sprintf("path '%s' does not exist", dirPath))
			return
		}

		if !dir.IsDir() {
			writeAPIErr(w, http.StatusBadRequest, fmt.Sprintf("path '%s' is not a directory", dirPath))
			return
		}

		basePath, dirName := path.Split(dirPath)
		tree := map[string]TreeItem{
			dirName: {IsDir: true, Name: dirName, Children: map[string]TreeItem{}},
		}

		if err = makeTree(basePath, tree); err != nil {
			writeAPIErr(w, http.StatusInternalServerError, "internal server error!")
			return
		}

		data := GetTreeResp{tree: tree}
		res := APIResponse[GetTreeResp]{OK: true, Data: &data}

		resBytes, err := json.Marshal(&res)
		if err != nil {
			writeAPIErr(w, http.StatusInternalServerError, "internal server error!")
			return
		}
		w.Write(resBytes)
	})
}

func writeAPIErr(w http.ResponseWriter, status int, error string) error {
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

type (
	TreeItem struct {
		Name     string
		IsDir    bool
		Children map[string]TreeItem
	}

	Tree = map[string]TreeItem
)

func makeTree(base string, tree Tree) error {
	for key, item := range tree {
		if !item.IsDir {
			continue
		}

		curr := path.Join(base, item.Name)
		childs, err := os.ReadDir(curr)
		if err != nil {
			return err
		}

		for _, child := range childs {
			tree[key].Children[child.Name()] = TreeItem{
				IsDir:    child.IsDir(),
				Name:     child.Name(),
				Children: map[string]TreeItem{},
			}
		}

		if err = makeTree(curr, tree[key].Children); err != nil {
			return err
		}
	}

	return nil
}
