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
	BaseAPIURL string
	c          *http.Client
}

// TODO add test to clients
func (gc GoSynClient) GetDirList(dirPath string) ([]handlers.DirChild, error) {
	res, err := gc.c.Get(gc.BaseAPIURL + "/api/dirs/list/" + dirPath)
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

func (gc GoSynClient) GetDirTree(dirPath string) (hutils.Tree, error) {
	res, err := gc.c.Get(gc.BaseAPIURL + "/api/dirs/tree/" + dirPath)
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

func (gc GoSynClient) GetFile(filePath string) (io.Reader, error) {
	res, err := gc.c.Get(gc.BaseAPIURL + "/api/files/" + filePath)
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

func (gc GoSynClient) PutNewFile(filePath string, isForced bool, reader io.Reader) error {
	req, err := http.NewRequest(http.MethodPut, gc.BaseAPIURL+"/api/files/new", reader)
	if err != nil {
		return err
	}

	req.Header.Set("x-file-path", filePath)
	if isForced {
		req.Header.Set("x-force", "true")
	} else {
		req.Header.Set("x-force", "false")
	}

	res, err := gc.c.Do(req)
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

func (gc GoSynClient) GetAllSpaces() ([]string, error) {
	res, err := gc.c.Get(gc.BaseAPIURL + "/api/spaces/all")
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
