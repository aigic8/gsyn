package client

import (
	"errors"
	"io"
	"net/http"

	"github.com/aigic8/gosyn/api/pb"
	"google.golang.org/protobuf/proto"
)

type GoSynClient struct {
	C *http.Client
}

// TODO add test to clients
func (gc *GoSynClient) GetDirList(baseAPIURL string, dirPath string) ([]*pb.DirChild, error) {
	res, err := gc.C.Get(baseAPIURL + "/api/dirs/list/" + dirPath)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		resBody, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}

		var resData pb.DirGetListResponse
		if err = proto.Unmarshal(resBody, &resData); err != nil {
			return nil, err
		}

		return resData.Children, nil
	}

	return nil, getErr(res)
}

func (gc *GoSynClient) GetDirTree(baseAPIURL string, dirPath string) (map[string]*pb.TreeItem, error) {
	res, err := gc.C.Get(baseAPIURL + "/api/dirs/tree/" + dirPath)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		resBody, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}

		var resData pb.DirGetTreeResponse
		if err = proto.Unmarshal(resBody, &resData); err != nil {
			return nil, err
		}

		return resData.Tree, nil
	}

	return nil, getErr(res)
}

func (gc *GoSynClient) GetFile(baseAPIURL string, filePath string) (io.Reader, error) {
	res, err := gc.C.Get(baseAPIURL + "/api/files/" + filePath)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, getErr(res)
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

	if res.StatusCode != http.StatusOK {
		return getErr(res)
	}

	return nil
}

func (gc *GoSynClient) GetMatches(baseAPIURL string, path string) ([]string, error) {
	res, err := gc.C.Get(baseAPIURL + "/api/files/matches/" + path)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		resBody, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}

		var resData pb.FileGetMatchResponse
		if err = proto.Unmarshal(resBody, &resData); err != nil {
			return nil, err
		}

		return resData.Matches, nil
	}

	return nil, getErr(res)
}

func (gc *GoSynClient) GetAllSpaces(baseAPIURL string) ([]string, error) {
	res, err := gc.C.Get(baseAPIURL + "/api/spaces/all")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		resBody, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}

		var resData pb.SpaceGetAllResponse
		if err = proto.Unmarshal(resBody, &resData); err != nil {
			return nil, err
		}

		return resData.Spaces, nil
	}

	return nil, getErr(res)
}

func (gc *GoSynClient) GetStat(baseAPIURL string, statPath string) (*pb.StatInfo, error) {
	res, err := gc.C.Get(baseAPIURL + "/api/files/stat/" + statPath)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		resBody, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}

		var resData pb.GetStatResponse
		if err = proto.Unmarshal(resBody, &resData); err != nil {
			return nil, err
		}

		return resData.Stat, nil
	}

	return nil, getErr(res)
}

func getErr(res *http.Response) error {
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	var resData pb.ApiError
	if err = proto.Unmarshal(resBody, &resData); err != nil {
		return err
	}

	return errors.New(resData.Message)
}
