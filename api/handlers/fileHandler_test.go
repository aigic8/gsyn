package handlers

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

	"github.com/aigic8/gosyn/api/handlers/handlerstest"
)

type fileGetTestCase struct {
	Name   string
	Status int
	Path   string
	Data   []byte
}

func TestFileGet(t *testing.T) {
	base := t.TempDir()

	err := handlerstest.MakeDirs(base, []string{
		"space/seethers/",
	})
	if err != nil {
		panic(err)
	}

	seetherTruthData := []byte("there is nothing you can say to salvage the lie.")
	err = handlerstest.MakeFiles(base, []handlerstest.FileInfo{
		{Path: "space/seethers/truth.txt", Data: seetherTruthData},
	})
	if err != nil {
		panic(err)
	}

	// TODO add following test cases:
	// - file path is for a directory
	// - file path is out of space
	// - file does not exist
	testCases := []fileGetTestCase{
		{Name: "normal", Status: http.StatusOK, Path: "seethers/truth.txt", Data: seetherTruthData},
	}

	spaces := map[string]string{
		"seethers": path.Join(base, "space/seethers"),
	}
	fileHander := FileHandler{Spaces: spaces}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			w := httptest.NewRecorder()

			r := httptest.NewRequest("GET", "/{path}", nil)
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("path", tc.Path)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			fileHander.Get(w, r)

			res := w.Result()
			defer res.Body.Close()

			resBody, err := io.ReadAll(res.Body)
			if err != nil {
				panic(err)
			}

			assert.Equal(t, res.StatusCode, tc.Status)
			assert.Equal(t, string(resBody), string(tc.Data))
		})
	}

}

type filePutNewTestCase struct {
	Name        string
	Status      int
	NewFilePath string
	NewFileData []byte
	RawFilePath string
}

func TestFilePutNew(t *testing.T) {
	base := t.TempDir()

	err := handlerstest.MakeDirs(base, []string{
		"space/pink-floyd/",
	})
	if err != nil {
		panic(err)
	}

	newFilePath := "pink-floyd/wish-you-were-here.txt"
	newFileData := []byte("Did you exchange; a walk-on part in the war; for a leading role in a cage?")

	// TODO add following test cases:
	// - directory of file does not exist
	// - file does exist in forced mode (x-force header is true)
	// - file does exist in normal mode
	// - path of file is a directory
	// - file path is out of space
	// - [OPTIONAL] space does not exist
	testCases := []filePutNewTestCase{
		{Name: "normal", Status: http.StatusOK, NewFilePath: newFilePath, NewFileData: newFileData, RawFilePath: "space/pink-floyd/wish-you-were-here.txt"},
	}

	spaces := map[string]string{"pink-floyd": path.Join(base, "space/pink-floyd")}
	fileHanlder := FileHandler{Spaces: spaces}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			w := httptest.NewRecorder()

			r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(tc.NewFileData))
			r.Header.Add("x-file-path", tc.NewFilePath)

			fileHanlder.PutNew(w, r)

			res := w.Result()
			assert.Equal(t, res.StatusCode, tc.Status)

			if tc.Status == http.StatusOK {
				_, err := os.Stat(path.Join(base, tc.RawFilePath))
				assert.Equal(t, err, nil)
			}
		})
	}
}
