package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/aigic8/gosyn/api/handlers/utils"
)

type SpaceHandler struct {
}
type (
	SpaceGetAllResp = utils.APIResponse[SpaceGetAllRespData]

	SpaceGetAllRespData struct {
		Spaces []string `json:"spaces"`
	}
)

func (h SpaceHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	userInfo := r.Context().Value(utils.UserContextKey).(*utils.UserInfo)

	spaces := make([]string, 0, len(userInfo.Spaces))
	for space := range userInfo.Spaces {
		spaces = append(spaces, space)
	}

	res := utils.APIResponse[SpaceGetAllRespData]{OK: true, Data: &SpaceGetAllRespData{Spaces: spaces}}
	resBytes, err := json.Marshal(&res)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error happened")
		return
	}

	w.Write(resBytes)
}
