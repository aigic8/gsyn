package utils

import (
	"net/http"
	"os"
	"path"
	"testing"

	"github.com/aigic8/gosyn/api/client"
	"github.com/stretchr/testify/assert"
)

type newDynamicPathTestCase struct {
	Name        string
	PathStr     string
	ErrExpected bool
	Expected    *DynamicPath
}

func TestNewDynamicPath(t *testing.T) {
	servers := map[string]*ServerInfo{
		"myserver": {
			Name:       "myserver",
			BaseAPIURL: "https://myserver.com",
			GUID:       "",
		},
		"yourserver": {
			Name:       "yourserver",
			BaseAPIURL: "https://yourserver.com",
			GUID:       "",
		},
	}

	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	localDP := &DynamicPath{
		IsRemote: false,
		Path:     "/home/projects/gsyn",
	}

	localRelativeDP := &DynamicPath{
		IsRemote: false,
		Path:     path.Join(cwd, "home/projects/gsyn"),
	}

	remoteDP := &DynamicPath{
		IsRemote: true,
		Server: &ServerInfo{
			Name:       "myserver",
			BaseAPIURL: servers["myserver"].BaseAPIURL,
			GUID:       "",
		},
		Path: "home/projects",
	}

	testCases := []newDynamicPathTestCase{
		{Name: "local", PathStr: "/home/projects/gsyn", ErrExpected: false, Expected: localDP},
		{Name: "localRelative", PathStr: "home/projects/gsyn", ErrExpected: false, Expected: localRelativeDP},
		{Name: "remote", PathStr: "myserver:home/projects", ErrExpected: false, Expected: remoteDP},
		{Name: "multiColons", PathStr: "myserver:yourserver:our/server", ErrExpected: true},
		{Name: "serverDoesNotExist", PathStr: "noserver:my/path", ErrExpected: true},
		{Name: "emptyPath", PathStr: "myserver:", ErrExpected: true},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			dPath, err := NewDynamicPath(tc.PathStr, cwd, servers)
			if tc.ErrExpected {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, dPath, tc.Expected)
			}
		})
	}
}

type dynamicPathCopyTestCase struct {
	Name          string
	From          *DynamicPath
	To            *DynamicPath
	ErrExpected   bool
	ExpectedFiles []string
}

func TestDynamicPathCopy(t *testing.T) {
	base := t.TempDir()

	err := MakeDirs(base, []string{"dist"})
	if err != nil {
		panic(err)
	}

	err = MakeFiles(base, []FileInfo{
		{Path: "app.txt", Data: []byte("HELLO THREE")},
		{Path: "dist/exist.txt", Data: []byte("I DO EXIST!")},
	})
	if err != nil {
		panic(err)
	}

	appPath := newLocalDP("app.txt", base)
	normalFiles := []string{path.Join(base, "app2.txt")}
	toDirFiles := []string{path.Join(base, "dist/app.txt")}
	testCases := []dynamicPathCopyTestCase{
		{Name: "normal", From: appPath, To: newLocalDP("app2.txt", base), ErrExpected: false, ExpectedFiles: normalFiles},
		{Name: "toDir", From: appPath, To: newLocalDP("dist", base), ErrExpected: false, ExpectedFiles: toDirFiles},
		{Name: "toDirDoesNotExist", From: appPath, To: newLocalDP("nowhere/app.txt", base), ErrExpected: true},
		{Name: "toAlreadyFile", From: appPath, To: newLocalDP("dist/exist.txt", base), ErrExpected: true},
	}

	gc := &client.GoSynClient{C: &http.Client{}}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			reader, _, err := tc.From.Reader(gc)
			if err != nil {
				panic(err)
			}

			err = tc.To.Copy(gc, path.Base(tc.From.Path), false, reader)
			if tc.ErrExpected {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				for _, file := range tc.ExpectedFiles {
					assert.True(t, assert.FileExists(t, file))
				}
			}
		})
	}
}

func newLocalDP(rawPath string, base string) *DynamicPath {
	dPath, err := NewDynamicPath(rawPath, base, map[string]*ServerInfo{})
	if err != nil {
		panic(err)
	}

	return dPath
}

type FileInfo struct {
	Path string
	Data []byte
}

func MakeFiles(base string, files []FileInfo) error {
	for _, file := range files {
		err := os.WriteFile(path.Join(base, file.Path), file.Data, 0777)
		if err != nil {
			return err
		}
	}

	return nil
}

func MakeDirs(base string, dirs []string) error {
	for _, dir := range dirs {
		err := os.MkdirAll(path.Join(base, dir), 0777)
		if err != nil {
			return err
		}
	}

	return nil
}
