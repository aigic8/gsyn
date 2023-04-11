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
	"google.golang.org/protobuf/proto"

	"github.com/aigic8/gosyn/api/handlers/handlerstest"
	"github.com/aigic8/gosyn/api/handlers/utils"
	"github.com/aigic8/gosyn/api/pb"
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
		{Path: "outsider.txt", Data: []byte("I am an outsider.")},
	})
	if err != nil {
		panic(err)
	}

	// TODO test Content-Length header
	// TODO add following test cases:
	// - file path is for a directory
	// - file does not exist
	// - user is unauthorized to access space
	testCases := []fileGetTestCase{
		{Name: "normal", Status: http.StatusOK, Path: "seethers/truth.txt", Data: seetherTruthData},
		{Name: "pathTraversal", Status: http.StatusUnauthorized, Path: "seethers/../../outsider.txt"},
	}

	spaces := map[string]string{
		"seethers": path.Join(base, "space/seethers"),
	}
	fileHandler := FileHandler{Spaces: spaces}

	userSpaces := map[string]bool{"seethers": true}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			w := httptest.NewRecorder()

			r := httptest.NewRequest(http.MethodGet, "/?path="+tc.Path, nil)
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("path", tc.Path)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			uInfo := utils.UserInfo{
				GUID:   "f3b1f1cb-d1e6-4700-8f96-c28182563729",
				Spaces: userSpaces,
			}
			ctx := context.WithValue(r.Context(), utils.UserContextKey, &uInfo)
			r = r.WithContext(ctx)

			fileHandler.Get(w, r)

			res := w.Result()
			defer res.Body.Close()

			resBody, err := io.ReadAll(res.Body)
			if err != nil {
				panic(err)
			}

			assert.Equal(t, res.StatusCode, tc.Status)
			if res.StatusCode == http.StatusOK {
				assert.Equal(t, string(resBody), string(tc.Data))
			}
		})
	}

}

type filePutNewTestCase struct {
	Name        string
	Status      int
	NewFilePath string
	SrcName     string
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

	pathTraversalPath := "pink-floyd/../../wish-you-were-here.txt"

	// TODO add following test cases:
	// - directory of file does not exist
	// - file does exist in forced mode (x-force header is true)
	// - file does exist in normal mode
	// - path of file is a directory
	// - user is unauthorized to access space
	// - [OPTIONAL] space does not exist
	testCases := []filePutNewTestCase{
		{Name: "normal", Status: http.StatusOK, NewFilePath: newFilePath, SrcName: "wish-you-were-here.txt", NewFileData: newFileData, RawFilePath: "space/pink-floyd/wish-you-were-here.txt"},
		{Name: "pathTraversal", Status: http.StatusUnauthorized, NewFilePath: pathTraversalPath, SrcName: "wish-you-were-here.txt", NewFileData: newFileData},
	}

	spaces := map[string]string{"pink-floyd": path.Join(base, "space/pink-floyd")}
	fileHandler := FileHandler{Spaces: spaces}

	userSpaces := map[string]bool{"pink-floyd": true}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			w := httptest.NewRecorder()

			r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(tc.NewFileData))
			r.Header.Add("x-file-path", tc.NewFilePath)
			r.Header.Add("x-src-name", tc.SrcName)

			uInfo := utils.UserInfo{
				GUID:   "f3b1f1cb-d1e6-4700-8f96-c28182563729",
				Spaces: userSpaces,
			}
			ctx := context.WithValue(r.Context(), utils.UserContextKey, &uInfo)
			r = r.WithContext(ctx)

			fileHandler.PutNew(w, r)

			res := w.Result()
			assert.Equal(t, res.StatusCode, tc.Status)

			if tc.Status == http.StatusOK {
				_, err := os.Stat(path.Join(base, tc.RawFilePath))
				assert.Equal(t, err, nil)
			}
		})
	}
}

type fileMatchTestCase struct {
	Name    string
	Status  int
	Pattern string
	Files   []string
}

func TestFileMatch(t *testing.T) {
	base := t.TempDir()

	err := handlerstest.MakeDirs(base, []string{"space/pink-floyd/special"})
	if err != nil {
		panic(err)
	}

	err = handlerstest.MakeFiles(base, []handlerstest.FileInfo{
		{Path: "space/pink-floyd/wish-you-were-here.txt", Data: []byte("hi")},
		{Path: "space/pink-floyd/time.txt", Data: []byte("hi")},
		{Path: "space/pink-floyd/wish-you-were-here.mp4", Data: []byte("hi")},
		{Path: "outsider.txt", Data: []byte("hello there")},
	})
	if err != nil {
		panic(err)
	}

	// TODO add test cases:
	// - space does not exist (maybe)
	// - path does not exist
	// - matches no file
	// - matches some files and dirs (should ignore the dirs)
	normalCaseFiles := []string{
		"pink-floyd/wish-you-were-here.txt",
		"pink-floyd/time.txt",
	}
	testCases := []fileMatchTestCase{
		{Name: "normal", Status: http.StatusOK, Pattern: "pink-floyd/*.txt", Files: normalCaseFiles},
		{Name: "pathTraversal", Status: http.StatusUnauthorized, Pattern: "pink-floyd/../.."},
	}

	spaces := map[string]string{
		"pink-floyd": path.Join(base, "space/pink-floyd"),
	}
	fileHandler := FileHandler{
		Spaces: spaces,
	}
	userSpaces := map[string]bool{"pink-floyd": true}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/?pattern="+tc.Pattern, nil)

			uInfo := utils.UserInfo{
				GUID:   "f3b1f1cb-d1e6-4700-8f96-c28182563729",
				Spaces: userSpaces,
			}
			ctx := context.WithValue(r.Context(), utils.UserContextKey, &uInfo)
			r = r.WithContext(ctx)

			fileHandler.Match(w, r)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, res.StatusCode, tc.Status)

			if res.StatusCode == http.StatusOK {
				resBody, err := io.ReadAll(res.Body)
				if err != nil {
					panic(err)
				}

				var resData pb.FileGetMatchResponse
				if err = proto.Unmarshal(resBody, &resData); err != nil {
					panic(err)
				}

				assert.ElementsMatch(t, resData.Matches, tc.Files)
			}
		})
	}

}

type fileStatTestCase struct {
	Name      string
	Status    int
	Path      string
	StatName  string
	StatIsDir bool
}

func TestFileStat(t *testing.T) {
	base := t.TempDir()

	err := handlerstest.MakeDirs(base, []string{
		"space/pink-floyd",
	})
	if err != nil {
		panic(err)
	}

	err = handlerstest.MakeFiles(base, []handlerstest.FileInfo{
		{Path: "space/pink-floyd/time.txt", Data: []byte("Plans that either come to naught or half a page of scribbled lines")},
		{Path: "outsider.txt", Data: []byte("I am an outsider.")},
	})
	if err != nil {
		panic(err)
	}

	// TODO add test cases:
	// - path is a dir
	// - path does not exist
	// - user is unauthorized to access space
	testCases := []fileStatTestCase{
		{Name: "normal", Status: http.StatusOK, Path: "pink-floyd/time.txt", StatName: "time.txt", StatIsDir: false},
		{Name: "pathTraversal", Status: http.StatusUnauthorized, Path: "pink-floyd/../../outsider.txt"},
	}

	spaces := map[string]string{
		"pink-floyd": path.Join(base, "space/pink-floyd"),
	}
	fileHandler := FileHandler{
		Spaces: spaces,
	}

	userSpaces := map[string]bool{"pink-floyd": true}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/?path="+tc.Path, nil)

			uInfo := utils.UserInfo{
				GUID:   "f3b1f1cb-d1e6-4700-8f96-c28182563729",
				Spaces: userSpaces,
			}
			ctx := context.WithValue(r.Context(), utils.UserContextKey, &uInfo)
			r = r.WithContext(ctx)

			fileHandler.Stat(w, r)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, res.StatusCode, tc.Status)

			if res.StatusCode == http.StatusOK {
				resBody, err := io.ReadAll(res.Body)
				if err != nil {
					panic(err)
				}

				var resData pb.GetStatResponse
				if err = proto.Unmarshal(resBody, &resData); err != nil {
					panic(err)
				}

				assert.Equal(t, resData.Stat.Name, tc.StatName)
				assert.Equal(t, resData.Stat.IsDir, tc.StatIsDir)
				assert.NotEqual(t, resData.Stat.Size, 0)
			}
		})
	}

}
