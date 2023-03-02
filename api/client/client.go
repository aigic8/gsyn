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
func (gc *GoSynClient) GetDirList(baseAPIURL, dirPath, GUID string) ([]*pb.DirChild, error) {
	req, err := http.NewRequest(http.MethodGet, baseAPIURL+"/api/dirs/list?path="+dirPath, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "simple "+GUID)

	res, err := gc.C.Do(req)
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

func (gc *GoSynClient) GetDirTree(baseAPIURL, dirPath, GUID string) (map[string]*pb.TreeItem, error) {
	req, err := http.NewRequest(http.MethodGet, baseAPIURL+"/api/dirs/tree?path="+dirPath, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "simple "+GUID)

	res, err := gc.C.Do(req)
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

func (gc *GoSynClient) GetFile(baseAPIURL, filePath, GUID string) (io.ReadCloser, int64, error) {
	req, err := http.NewRequest(http.MethodGet, baseAPIURL+"/api/files?path="+filePath, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "simple "+GUID)

	res, err := gc.C.Do(req)
	if err != nil {
		return nil, 0, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, 0, getErr(res)
	}

	return res.Body, res.ContentLength, nil
}

func (gc *GoSynClient) PutNewFile(baseAPIURL, filePath, GUID, srcName string, isForced bool, reader io.Reader) error {
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
	req.Header.Set("x-src-name", srcName)
	req.Header.Set("Authorization", "simple "+GUID)

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

func (gc *GoSynClient) GetMatches(baseAPIURL, GUID, pattern string) ([]string, error) {
	req, err := http.NewRequest(http.MethodGet, baseAPIURL+"/api/files/matches?pattern="+pattern, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "simple "+GUID)

	res, err := gc.C.Do(req)
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

func (gc *GoSynClient) GetAllSpaces(baseAPIURL, GUID string) ([]string, error) {
	req, err := http.NewRequest(http.MethodGet, baseAPIURL+"/api/spaces/all", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "simple "+GUID)

	res, err := gc.C.Do(req)
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

func (gc *GoSynClient) GetStat(baseAPIURL, GUID, statPath string) (*pb.StatInfo, error) {
	req, err := http.NewRequest(http.MethodGet, baseAPIURL+"/api/files/stat?path="+statPath, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "simple "+GUID)

	res, err := gc.C.Do(req)
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
