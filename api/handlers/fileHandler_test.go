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
	Name          string
	Status        int
	Path          string
	Data          []byte
	ContentLength int64
}

func TestFileGet(t *testing.T) {
	base := t.TempDir()

	err := handlerstest.MakeDirs(base, []string{
		"space/seethers/dir",
		"space/pink-floyd",
	})
	if err != nil {
		panic(err)
	}

	seetherTruthData := []byte("there is nothing you can say to salvage the lie.")
	err = handlerstest.MakeFiles(base, []handlerstest.FileInfo{
		{Path: "space/seethers/truth.txt", Data: seetherTruthData},
		{Path: "space/pink-floyd/wish-you-were-here.txt", Data: []byte("Did they get you to trade; Your heroes for ghosts?")},
		{Path: "outsider.txt", Data: []byte("I am an outsider.")},
	})
	if err != nil {
		panic(err)
	}

	normalStat, err := os.Stat(path.Join(base, "space/seethers/truth.txt"))
	if err != nil {
		panic(err)
	}

	testCases := []fileGetTestCase{
		{Name: "normal", Status: http.StatusOK, Path: "seethers/truth.txt", Data: seetherTruthData, ContentLength: normalStat.Size()},
		{Name: "unauthorizedSpace", Status: http.StatusUnauthorized, Path: "pink-floyd/wish-you-were-here.txt"},
		{Name: "pathTraversal", Status: http.StatusUnauthorized, Path: "seethers/../../outsider.txt"},
		{Name: "pathIsDir", Status: http.StatusBadRequest, Path: "seethers/dir"},
		{Name: "notExists", Status: http.StatusNotFound, Path: "seethers/what-would-you-do.txt"},
	}

	spaces := map[string]string{
		"seethers":   path.Join(base, "space/seethers"),
		"pink-floyd": path.Join(base, "space/pink-floyd"),
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

			assert.Equal(t, res.StatusCode, tc.Status)
			if res.StatusCode == http.StatusOK {
				resBody, err := io.ReadAll(res.Body)
				if err != nil {
					panic(err)
				}

				assert.Equal(t, res.ContentLength, tc.ContentLength)
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
	IsForce     bool
}

func TestFilePutNew(t *testing.T) {
	base := t.TempDir()

	err := handlerstest.MakeDirs(base, []string{
		"space/pink-floyd/old",
		"space/seethers/",
	})
	if err != nil {
		panic(err)
	}

	err = handlerstest.MakeFiles(base, []handlerstest.FileInfo{
		{Path: "space/pink-floyd/time.txt", Data: []byte("The time is gone, the song is over, thought I'd something more to say")},
	})
	if err != nil {
		panic(err)
	}

	newFilePath := "pink-floyd/wish-you-were-here.txt"
	newFileData := []byte("Did you exchange; a walk-on part in the war; for a leading role in a cage?")

	pathTraversalPath := "pink-floyd/../../wish-you-were-here.txt"

	// TODO add following test cases:
	// - [OPTIONAL] space does not exist
	fileExistsForceFileData := []byte("Plans that either come to naught or half a page of scribbled lines")
	testCases := []filePutNewTestCase{
		{Name: "normal", Status: http.StatusOK, NewFilePath: newFilePath, SrcName: "wish-you-were-here.txt", NewFileData: newFileData, RawFilePath: "space/pink-floyd/wish-you-were-here.txt"},
		{Name: "directoryDoesNotExist", Status: http.StatusBadRequest, NewFilePath: "pink-floyd/special/wish-you-were-here.txt", SrcName: "wish-you-were-here.txt", NewFileData: newFileData},
		{Name: "pathTraversal", Status: http.StatusUnauthorized, NewFilePath: pathTraversalPath, SrcName: "wish-you-were-here.txt", NewFileData: newFileData},
		{Name: "fileExistsNormal", Status: http.StatusBadRequest, NewFilePath: "pink-floyd/time.txt", SrcName: "time.txt", NewFileData: []byte("Home, home again.")},
		{Name: "fileExistsForce", Status: http.StatusOK, NewFilePath: "pink-floyd/time.txt", SrcName: "time.txt", NewFileData: fileExistsForceFileData, RawFilePath: "space/pink-floyd/time.txt", IsForce: true},
		{Name: "fileIsDir", Status: http.StatusBadRequest, NewFilePath: "pink-floyd", SrcName: "old", NewFileData: newFileData},
		{Name: "unauthorizedSpace", Status: http.StatusUnauthorized, NewFilePath: "seethers/truth.txt", SrcName: "truth.txt", NewFileData: []byte("No, there's nothing you say that can salvage the lie")},
	}

	spaces := map[string]string{
		"pink-floyd": path.Join(base, "space/pink-floyd"),
		"seethers":   path.Join(base, "space/seethers"),
	}
	fileHandler := FileHandler{Spaces: spaces}

	userSpaces := map[string]bool{"pink-floyd": true}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			w := httptest.NewRecorder()

			r := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(tc.NewFileData))
			r.Header.Add("x-file-path", tc.NewFilePath)
			r.Header.Add("x-src-name", tc.SrcName)
			if tc.IsForce {
				r.Header.Add("x-force", "true")
			}

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
				newFilePath := path.Join(base, tc.RawFilePath)
				newFile, err := os.Open(newFilePath)
				assert.Nil(t, err)
				defer newFile.Close()

				fileBytes, err := io.ReadAll(newFile)
				assert.Nil(t, err)
				assert.Equal(t, tc.NewFileData, fileBytes)
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

	err := handlerstest.MakeDirs(base, []string{"space/pink-floyd/special", "space/pink-floyd/data-4.zip"})
	if err != nil {
		panic(err)
	}

	err = handlerstest.MakeFiles(base, []handlerstest.FileInfo{
		{Path: "space/pink-floyd/wish-you-were-here.txt", Data: []byte("hi")},
		{Path: "space/pink-floyd/time.txt", Data: []byte("hi")},
		{Path: "space/pink-floyd/wish-you-were-here.mp4", Data: []byte("hi")},
		{Path: "space/pink-floyd/data-5.zip", Data: []byte("hello")},
		{Path: "outsider.txt", Data: []byte("hello there")},
	})
	if err != nil {
		panic(err)
	}

	// TODO add test cases:
	// - space does not exist (maybe)
	normalCaseFiles := []string{
		"pink-floyd/wish-you-were-here.txt",
		"pink-floyd/time.txt",
	}
	ignoreDirsCaseFiles := []string{
		"pink-floyd/data-5.zip",
	}
	testCases := []fileMatchTestCase{
		{Name: "normal", Status: http.StatusOK, Pattern: "pink-floyd/*.txt", Files: normalCaseFiles},
		{Name: "fileNotExist", Status: http.StatusNotFound, Pattern: "pink-floyd/special/high-hopes.txt"},
		{Name: "patternNoMatches", Status: http.StatusNotFound, Pattern: "pink-floyd/*.md"},
		{Name: "ignoreDirs", Status: http.StatusOK, Pattern: "pink-floyd/data-*.zip", Files: ignoreDirsCaseFiles},
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
		"space/pink-floyd/special",
		"space/seethers",
	})
	if err != nil {
		panic(err)
	}

	err = handlerstest.MakeFiles(base, []handlerstest.FileInfo{
		{Path: "space/pink-floyd/time.txt", Data: []byte("Plans that either come to naught or half a page of scribbled lines")},
		{Path: "space/seethers/truth.txt", Data: []byte("The deception you show is your own parasite")},
		{Path: "outsider.txt", Data: []byte("I am an outsider.")},
	})
	if err != nil {
		panic(err)
	}

	testCases := []fileStatTestCase{
		{Name: "normal", Status: http.StatusOK, Path: "pink-floyd/time.txt", StatName: "time.txt", StatIsDir: false},
		{Name: "pathTraversal", Status: http.StatusUnauthorized, Path: "pink-floyd/../../outsider.txt"},
		{Name: "dir", Status: http.StatusOK, Path: "pink-floyd/special", StatName: "special", StatIsDir: true},
		{Name: "notExists", Status: http.StatusNotFound, Path: "pink-floyd/wish-you-were-here.txt"},
		{Name: "unauthorizedSpace", Status: http.StatusUnauthorized, Path: "seethers/truth.txt"},
	}

	spaces := map[string]string{
		"pink-floyd": path.Join(base, "space/pink-floyd"),
		"seethers":   path.Join(base, "space/seethers"),
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
