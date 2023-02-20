package client

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/aigic8/gosyn/api/handlers"
	hutils "github.com/aigic8/gosyn/api/handlers/utils"
)

type GoSynClient struct {
	C *http.Client
}

// TODO add test to clients
func (gc *GoSynClient) GetDirList(baseAPIURL string, dirPath string) ([]handlers.DirChild, error) {
	res, err := gc.C.Get(baseAPIURL + "/api/dirs/list/" + dirPath)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var resData handlers.DirGetListResp
	if err = json.Unmarshal(resBody, &resData); err != nil {
		return nil, err
	}

	if !resData.OK {
		return nil, errors.New("none-ok response: " + resData.Error)
	}

	return resData.Data.Children, nil
}

func (gc *GoSynClient) GetDirTree(baseAPIURL string, dirPath string) (hutils.Tree, error) {
	res, err := gc.C.Get(baseAPIURL + "/api/dirs/tree/" + dirPath)
	if err != nil {
		return hutils.Tree{}, err
	}

	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var resData handlers.DirGetTreeResp
	if err = json.Unmarshal(resBody, &resData); err != nil {
		return nil, err
	}

	if !resData.OK {
		return nil, errors.New("none-ok response: " + resData.Error)
	}

	return resData.Data.Tree, nil
}

func (gc *GoSynClient) GetFile(baseAPIURL string, filePath string) (io.Reader, error) {
	res, err := gc.C.Get(baseAPIURL + "/api/files/" + filePath)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}

		var resData hutils.APIResponse[map[string]bool]
		if err = json.Unmarshal(resBody, &resData); err != nil {
			return nil, err
		}

		return nil, errors.New("none-ok status: " + resData.Error)
	}

	return res.Body, nil
}

func (gc *GoSynClient) PutNewFile(baseAPIURL string, filePath string, isForced bool, reader io.Reader) error {
	req, err := http.NewRequest(http.MethodPut, baseAPIURL+"/api/files/new", reader)
	if err != nil {
		return err
	}

	req.Header.Set("x-file-path", filePath)
	if isForced {
		req.Header.Set("x-force", "true")
	} else {
		req.Header.Set("x-force", "false")
	}

	res, err := gc.C.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	var resData hutils.APIResponse[map[string]bool]
	if err = json.Unmarshal(resBody, &resData); err != nil {
		return err
	}

	if !resData.OK {
		return errors.New("none-ok response: " + resData.Error)
	}

	return nil
}

func (gc *GoSynClient) GetMatches(baseAPIURL string, path string) ([]string, error) {
	res, err := gc.C.Get(baseAPIURL + "/api/files/matches" + path)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var resData handlers.FileGetMatchResp
	if err = json.Unmarshal(resBody, &resData); err != nil {
		return nil, err
	}

	return resData.Data.Matches, nil
}

func (gc *GoSynClient) GetAllSpaces(baseAPIURL string) ([]string, error) {
	res, err := gc.C.Get(baseAPIURL + "/api/spaces/all")
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var resData handlers.SpaceGetAllResp
	if err = json.Unmarshal(resBody, &resData); err != nil {
		return nil, err
	}

	if !resData.OK {
		return nil, errors.New("none-ok response: " + resData.Error)
	}

	return resData.Data.Spaces, nil
}

func (gc *GoSynClient) GetStat(baseAPIURL string, statPath string) (handlers.StatInfo, error) {
	res, err := gc.C.Get(baseAPIURL + "/api/files/stat/" + statPath)
	if err != nil {
		return handlers.StatInfo{}, err
	}

	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return handlers.StatInfo{}, err
	}

	var resData handlers.FileGetStatResp
	if err = json.Unmarshal(resBody, &resData); err != nil {
		return handlers.StatInfo{}, err
	}

	if !resData.OK {
		return handlers.StatInfo{}, errors.New("none-ok response: " + resData.Error)
	}

	return resData.Data.Stat, nil

}
