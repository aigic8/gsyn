package handlers

import (
	"net/http"

	"github.com/aigic8/gosyn/api/handlers/utils"
	"github.com/aigic8/gosyn/api/pb"
	"google.golang.org/protobuf/proto"
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

	res := pb.SpaceGetAllResponse{Spaces: spaces}
	resBytes, err := proto.Marshal(&res)
	if err != nil {
		utils.WriteAPIErr(w, http.StatusInternalServerError, "internal server error happened")
		return
	}

	w.Write(resBytes)
}
