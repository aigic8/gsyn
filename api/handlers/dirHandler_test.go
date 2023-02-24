package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"

	"github.com/aigic8/gosyn/api/handlers/handlerstest"
	"github.com/aigic8/gosyn/api/handlers/utils"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

type getDirListTestCase struct {
	Name   string
	Status int
	Path   string
	Resp   DirGetListRespData
}

func TestDirGetList(t *testing.T) {
	base := t.TempDir()

	err := handlerstest.MakeDirs(base, []string{
		"space/seethers/special",
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

	normalResp := DirGetListRespData{
		Children: []DirChild{
			{Name: "special", IsDir: true},
			{Name: "truth.txt", IsDir: false},
		},
	}

	// TODO add test caeses:
	// - path is a file
	// - path does not exist
	// - path is out of space
	// - user is unauthorized
	testCases := []getDirListTestCase{
		{Name: "normal", Status: http.StatusOK, Path: "seethers", Resp: normalResp},
	}

	spaces := map[string]string{
		"seethers": path.Join(base, "space/seethers"),
	}
	dirHandler := DirHandler{Spaces: spaces}

	userSpaces := map[string]bool{"seethers": true}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			w := httptest.NewRecorder()

			r := httptest.NewRequest("GET", "/{path}", nil)
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("path", tc.Path)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			uInfo := utils.UserInfo{
				GUID:   "f3b1f1cb-d1e6-4700-8f96-c28182563729",
				Spaces: userSpaces,
			}
			ctx := context.WithValue(r.Context(), utils.UserContextKey, &uInfo)
			r = r.WithContext(ctx)

			dirHandler.GetList(w, r)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, res.StatusCode, tc.Status)

			resBody, err := io.ReadAll(res.Body)
			if err != nil {
				panic(err)
			}

			t.Log(string(resBody))

			var resData utils.APIResponse[DirGetListRespData]
			if err = json.Unmarshal(resBody, &resData); err != nil {
				panic(err)
			}

			assert.Equal(t, resData.OK, true)
			assert.Equal(t, resData.Data, &tc.Resp)
		})
	}
}

type getDirTreeTestCase struct {
	Name   string
	Status int
	Path   string
	Tree   utils.Tree
}

func TestDirGetTree(t *testing.T) {
	base := t.TempDir()

	err := handlerstest.MakeDirs(base, []string{
		"space/seethers/special",
	})
	if err != nil {
		panic(err)
	}

	seetherTruthData := []byte("there is nothing you can say to salvage the lie.")
	seetherSaveTodayData := []byte("So save the secrets that you prayed for; Awake; I'll see you on the other side.")
	err = handlerstest.MakeFiles(base, []handlerstest.FileInfo{
		{Path: "space/seethers/truth.txt", Data: seetherTruthData},
		{Path: "space/seethers/special/save-today.txt", Data: seetherSaveTodayData},
	})
	if err != nil {
		panic(err)
	}

	normalBase := path.Join(base, "space/seethers")
	normalTree := utils.Tree{
		"seethers": utils.TreeItem{
			Path:  normalBase,
			IsDir: true,
			Children: map[string]utils.TreeItem{
				"special": {
					Path:  path.Join(normalBase, "/special"),
					IsDir: true,
					Children: map[string]utils.TreeItem{
						"save-today.txt": {Path: path.Join(normalBase, "/special/save-today.txt"), IsDir: false, Children: map[string]utils.TreeItem{}},
					},
				},
				"truth.txt": {
					Path:     path.Join(normalBase, "/truth.txt"),
					IsDir:    false,
					Children: map[string]utils.TreeItem{},
				},
			},
		},
	}

	// TODO add test caeses:
	// - path is a file
	// - path does not exist
	// - path is out of space
	// - user is unauthorized
	testCases := []getDirTreeTestCase{
		{Name: "normal", Status: http.StatusOK, Path: "seethers", Tree: normalTree},
	}

	spaces := map[string]string{
		"seethers": path.Join(base, "space/seethers"),
	}
	dirHandler := DirHandler{Spaces: spaces}

	userSpaces := map[string]bool{"seethers": true}

	for _, tc := range testCases {
		w := httptest.NewRecorder()

		r := httptest.NewRequest("GET", "/{path}", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("path", tc.Path)
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		uInfo := utils.UserInfo{
			GUID:   "f3b1f1cb-d1e6-4700-8f96-c28182563729",
			Spaces: userSpaces,
		}
		ctx := context.WithValue(r.Context(), utils.UserContextKey, &uInfo)
		r = r.WithContext(ctx)

		dirHandler.GetTree(w, r)

		res := w.Result()
		defer res.Body.Close()

		assert.Equal(t, res.StatusCode, tc.Status)

		resBody, err := io.ReadAll(res.Body)
		if err != nil {
			panic(err)
		}

		var resData utils.APIResponse[DirGetTreeRespData]
		if err = json.Unmarshal(resBody, &resData); err != nil {
			panic(err)
		}

		assert.Equal(t, resData.OK, true)
		assert.Equal(t, resData.Data.Tree, tc.Tree)
	}

}
