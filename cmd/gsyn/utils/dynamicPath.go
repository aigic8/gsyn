package utils

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aigic8/gosyn/api/client"
)

type (
	DynamicPath struct {
		IsRemote bool
		Server   *ServerInfo
		Path     string
	}

	ServerInfo struct {
		Name         string
		BaseAPIURL   string
		GUID         string
		Certificates []string
	}
)

func NewDynamicPath(rawPath string, base string, servers map[string]*ServerInfo) (*DynamicPath, error) {
	pathParts := strings.Split(rawPath, ":")
	pathPartsLen := len(pathParts)
	if pathPartsLen > 2 {
		return nil, errors.New("more than one colon")
	}

	if pathPartsLen == 1 {
		localPath := pathParts[0]
		if !strings.HasPrefix(localPath, "/") {
			localPath = path.Join(base, rawPath)
		}
		return &DynamicPath{
			IsRemote: false,
			Path:     localPath,
		}, nil
	}

	// pathPartsLen == 2
	_, serverExists := servers[pathParts[0]]
	if !serverExists {
		return nil, fmt.Errorf("server '%s' does not exist", pathParts[0])
	}

	if pathParts[1] == "" {
		return nil, fmt.Errorf("empty path")
	}
	return &DynamicPath{
		IsRemote: true,
		Server:   servers[pathParts[0]],
		Path:     pathParts[1],
	}, nil
}

type StatInfo struct {
	Name    string
	IsDir   bool
	ModTime time.Time
	Size    int64
}

func (dPath *DynamicPath) Stat(gc *client.GoSynClient) (*StatInfo, error) {
	if !dPath.IsRemote {
		stat, err := os.Stat(dPath.Path)
		if err != nil {
			return nil, err
		}

		return &StatInfo{
			Name:    stat.Name(),
			IsDir:   stat.IsDir(),
			ModTime: stat.ModTime(),
			Size:    stat.Size(),
		}, nil
	}

	statInfo, err := gc.GetStat(dPath.Server.BaseAPIURL, dPath.Server.GUID, dPath.Path)
	if err != nil {
		return nil, err
	}

	return &StatInfo{
		Name:    statInfo.Name,
		IsDir:   statInfo.IsDir,
		ModTime: statInfo.ModTime.AsTime(),
		Size:    statInfo.Size,
	}, nil

}

func (dPath *DynamicPath) GetMatches(gc *client.GoSynClient) ([]*DynamicPath, error) {
	if !dPath.IsRemote {
		matches, err := filepath.Glob(dPath.Path)
		if err != nil {
			return nil, fmt.Errorf("malformed pattern: %w", err)
		}

		fileMatches := []*DynamicPath{}
		for _, match := range matches {
			stat, err := os.Stat(match)
			if err != nil {
				return fileMatches, fmt.Errorf("error stating path '%s': %w", match, err)
			}

			if !stat.IsDir() {
				fileMatches = append(fileMatches, &DynamicPath{IsRemote: false, Path: dPath.Path})
			}
		}

		if len(fileMatches) == 0 && !isPatternLike(dPath.Path) {
			return nil, fmt.Errorf("no file or directory '%s'", dPath.Path)
		}

		return fileMatches, nil
	}

	matchesStr, err := gc.GetMatches(dPath.Server.BaseAPIURL, dPath.Server.GUID, dPath.Path)
	if err != nil {
		return nil, fmt.Errorf("error getting matches for '%s': %w", dPath.Path, err)
	}

	if len(matchesStr) == 0 && !isPatternLike(dPath.Path) {
		return nil, fmt.Errorf("no file matched path '%s'", dPath.Path)
	}

	fileMatches := make([]*DynamicPath, 0, len(matchesStr))
	for _, match := range matchesStr {
		fileMatches = append(fileMatches, &DynamicPath{
			IsRemote: true,
			Server:   dPath.Server,
			Path:     match,
		})
	}

	return fileMatches, nil
}

func isPatternLike(path string) bool {
	return strings.ContainsRune(path, '?') || strings.ContainsRune(path, '*')
}

func (dPath *DynamicPath) Reader(gc *client.GoSynClient) (io.ReadCloser, int64, error) {
	if !dPath.IsRemote {
		file, err := os.Open(dPath.Path)
		if err != nil {
			return nil, -1, err
		}

		stat, err := file.Stat()
		if err != nil {
			return nil, -1, err
		}
		return file, stat.Size(), nil
	}

	return gc.GetFile(dPath.Server.BaseAPIURL, dPath.Path, dPath.Server.GUID)
}

func (dPath *DynamicPath) Copy(gc *client.GoSynClient, srcName string, force bool, reader io.Reader) error {
	if !dPath.IsRemote {
		writeDest := dPath.Path
		writeStat, err := os.Stat(dPath.Path)
		destExist := !errors.Is(err, os.ErrNotExist)
		if err != nil && destExist {
			return err
		}

		if err == nil && writeStat.IsDir() {
			writeDest = path.Join(dPath.Path, srcName)
			writeStat, err = os.Stat(writeDest)
			destExist = !errors.Is(err, os.ErrNotExist)
			if err != nil && destExist {
				return err
			}

			if err == nil && writeStat.IsDir() {
				return fmt.Errorf("path '%s' is a directory", writeDest)
			}
		}

		if destExist && !force {
			return fmt.Errorf("file '%s' already exists", writeDest)
		}

		w, err := os.Create(writeDest)
		if err != nil {
			return err
		}

		if _, err = io.Copy(w, reader); err != nil {
			return err
		}

		return nil
	}

	return gc.PutNewFile(dPath.Server.BaseAPIURL, dPath.Path, dPath.Server.GUID, srcName, force, reader)
}

func (dPath *DynamicPath) String() string {
	if dPath.IsRemote {
		return dPath.Server.Name + ":" + dPath.Path
	}
	return dPath.Path
}
