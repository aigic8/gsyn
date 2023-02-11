package handlerstest

import (
	"os"
	"path"
)

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
