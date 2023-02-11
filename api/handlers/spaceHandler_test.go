package handlers

import (
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

	spacesMap := map[string]string{
		"spiderman": "home/spiderman",
		"batman":    "home/projects/batman",
	}

	spacesArr := make([]string, 0, len(spacesMap))
	for space := range spacesMap {
		spacesArr = append(spacesArr, space)
	}

	testCases := []spaceGetAllTestCase{
		{Name: "normal", Status: http.StatusOK, Spaces: spacesArr},
	}

	spaceHandler := SpaceHandler{Spaces: spacesMap}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)

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
