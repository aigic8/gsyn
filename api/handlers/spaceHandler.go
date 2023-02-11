package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/aigic8/gosyn/api/handlers/utils"
)

type SpaceHandler struct {
	Spaces map[string]string
}

type GetAllRespData struct {
	Spaces []string `json:"spaces"`
}

func (h SpaceHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	spaces := make([]string, 0, len(h.Spaces))
	for space := range h.Spaces {
		spaces = append(spaces, space)
	}

	res := utils.APIResponse[GetAllRespData]{OK: true, Data: &GetAllRespData{Spaces: spaces}}
	resBytes, err := json.Marshal(&res)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error happened")
		return
	}

	w.Write(resBytes)
}
