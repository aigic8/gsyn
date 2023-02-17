package utils

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

type PathValidator struct {
	Path   string
	Stat   fs.FileInfo
	exists bool
	err    error
}

func NewPathValidator(path string) (*PathValidator, error) {
	stat, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &PathValidator{
				Path:   path,
				Stat:   stat,
				exists: false,
				err:    nil,
			}, nil
		}

		return nil, err
	}

	return &PathValidator{
		Path:   path,
		Stat:   stat,
		exists: true,
		err:    nil,
	}, nil
}

func (pv *PathValidator) Reset() *PathValidator {
	pv.err = nil
	return pv
}
func (pv *PathValidator) Exist() *PathValidator {
	if pv.err == nil && !pv.exists {
		pv.err = fmt.Errorf("path '%s' does not exist", pv.Path)
	}
	return pv
}

func (pv *PathValidator) NotExist() *PathValidator {
	if pv.err == nil && pv.exists {
		pv.err = fmt.Errorf("path '%s' exists", pv.Path)
	}
	return pv
}

func (pv *PathValidator) Dir() *PathValidator {
	if pv.err == nil && !pv.Stat.IsDir() {
		pv.err = fmt.Errorf("path '%s' is not a directory", pv.Path)
	}
	return pv
}

func (pv *PathValidator) File() *PathValidator {
	if pv.err == nil && pv.Stat.IsDir() {
		pv.err = fmt.Errorf("path '%s' is a directory", pv.Path)
	}
	return pv
}

func (pv *PathValidator) Result() error {
	return pv.err
}
