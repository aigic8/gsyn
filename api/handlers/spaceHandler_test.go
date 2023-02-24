package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aigic8/gosyn/api/handlers/utils"
	"github.com/stretchr/testify/assert"
)

type spaceGetAllTestCase struct {
	Name   string
	Status int
	Spaces []string
}

func TestSpaceGetAll(t *testing.T) {

	userSpaces := map[string]bool{"spiderman": true, "batman": true}
	spaces := make([]string, 0, len(userSpaces))
	for space := range userSpaces {
		spaces = append(spaces, space)
	}

	testCases := []spaceGetAllTestCase{
		{Name: "normal", Status: http.StatusOK, Spaces: spaces},
	}

	spaceHandler := SpaceHandler{}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)

			uInfo := utils.UserInfo{
				GUID:   "f3b1f1cb-d1e6-4700-8f96-c28182563729",
				Spaces: userSpaces,
			}
			ctx := context.WithValue(r.Context(), utils.UserContextKey, &uInfo)
			r = r.WithContext(ctx)

			spaceHandler.GetAll(w, r)

			res := w.Result()
			defer res.Body.Close()
			assert.Equal(t, res.StatusCode, tc.Status)

			resBody, err := io.ReadAll(res.Body)
			if err != nil {
				panic(err)
			}

			var resData utils.APIResponse[SpaceGetAllRespData]
			if err := json.Unmarshal(resBody, &resData); err != nil {
				panic(err)
			}

			assert.Equal(t, resData.OK, true)
			assert.ElementsMatch(t, resData.Data.Spaces, tc.Spaces)
		})
	}

}
