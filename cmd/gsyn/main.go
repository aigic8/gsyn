package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aigic8/gosyn/api/client"
	"github.com/aigic8/gosyn/cmd/gsyn/utils"
	"github.com/alexflint/go-arg"
)

type (
	args struct {
		Cp *cpArgs `arg:"subcommand:cp"`
	}

	cpArgs struct {
		Force   bool     `arg:"-f"`
		Workers int      `arg:"-w,--workers"`
		Paths   []string `arg:"positional"`
		Timeout int      `arg:"-t,--timeout"`
	}
)

type DynamicPath struct {
	IsRemote   bool
	IsForce    bool
	BaseAPIURL string
	ServerName string
	Path       string
}

func main() {
	var args args
	arg.MustParse(&args)
	// TODO
	servers := map[string]string{}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if args.Cp != nil {
		pathsLen := len(args.Cp.Paths)
		if pathsLen < 2 {
			fmt.Fprintln(os.Stderr, "need at least a source and destination path")
			os.Exit(1)
		}

		srcs := make([]DynamicPath, 0, pathsLen-1)
		for _, rawPath := range args.Cp.Paths[:pathsLen-1] {
			dPath, err := ParseDynamicPath(rawPath, cwd, servers)
			if err != nil {
				fmt.Fprintf(os.Stderr, "malformed path: %v\n", err)
				os.Exit(1)
			}
			srcs = append(srcs, dPath)
			// FIXME validate paths
		}

		// TODO for now dest only can be local
		dest, err := ParseDynamicPath(args.Cp.Paths[pathsLen-1], cwd, servers)
		if err != nil {
			fmt.Fprintf(os.Stderr, "malformed path: %v\n", err)
			os.Exit(1)
		}
		// FIXME validate path

		// destDirMode is when we destinition is a directory and copy files to that (destination is not the actual file path)
		destDirMode := false
		if !dest.IsRemote {
			parentValidator, err := utils.NewPathValidator(path.Dir(dest.Path))
			if err != nil {
				fmt.Fprintf(os.Stderr, "error getting info: %s", err.Error())
				os.Exit(1)
			}

			if err = parentValidator.Exist().Dir().Result(); err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(1)
			}

			destValidator, err := utils.NewPathValidator(dest.Path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error getting info: %s", err.Error())
				os.Exit(1)
			}

			dirErr := destValidator.Exist().Dir().Result()
			destDirMode = dirErr == nil
			if len(srcs) > 1 && dirErr != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(1)
			}
		}

		mw := NewMassWriter(time.Duration(args.Cp.Timeout)*time.Millisecond, &dest, destDirMode)

		srcsChan := make(chan *DynamicPath)
		matchesChan := make(chan *DynamicPath)

		matchesWg := new(sync.WaitGroup)
		matchesWg.Add(args.Cp.Workers)
		for i := 0; i < args.Cp.Workers; i++ {
			go mw.GetMatches(srcsChan, matchesChan, matchesWg)
		}

		go func() {
			for _, src := range srcs {
				srcsChan <- &src
			}
			close(srcsChan)
			matchesWg.Wait()
			close(matchesChan)
		}()

		writesWg := new(sync.WaitGroup)
		writesWg.Add(args.Cp.Workers)
		for i := 0; i < args.Cp.Workers; i++ {
			mw.WriteToDest(matchesChan, writesWg)
		}

		writesWg.Wait()
	}

}

func ParseDynamicPath(rawPath string, base string, servers map[string]string) (DynamicPath, error) {
	pathParts := strings.Split(rawPath, ":")
	pathPartsLen := len(pathParts)
	if pathPartsLen > 2 {
		return DynamicPath{}, errors.New("more than one collon")
	}

	if pathPartsLen == 1 {
		localPath := pathParts[0]
		if !strings.HasPrefix(localPath, "/") {
			localPath = path.Join(base, rawPath)
		}
		return DynamicPath{
			IsRemote: false,
			Path:     localPath,
		}, nil
	}

	// pathPartsLen == 2
	baseAPIURL, serverExists := servers[pathParts[0]]
	if !serverExists {
		return DynamicPath{}, fmt.Errorf("server '%s' does not exist", pathParts[0])
	}
	return DynamicPath{
		IsRemote:   true,
		ServerName: pathParts[0],
		BaseAPIURL: baseAPIURL,
		Path:       pathParts[1],
	}, nil
}

type MassWriter struct {
	c       *http.Client
	Dest    *DynamicPath
	DirMode bool
}

func NewMassWriter(timeout time.Duration, dest *DynamicPath, dirMode bool) *MassWriter {
	return &MassWriter{
		c: &http.Client{
			Timeout: timeout,
		},
		Dest:    dest,
		DirMode: dirMode,
	}
}

type Match struct {
	IsRemote   bool
	BaseAPIURL string
	Matches    []string
}

func (mw *MassWriter) GetMatches(dPaths <-chan *DynamicPath, out chan<- *DynamicPath, wg *sync.WaitGroup) {
	defer wg.Done()
	for dPath := range dPaths {
		if dPath.IsRemote {

			gsync := client.GoSynClient{
				BaseAPIURL: dPath.BaseAPIURL,
				C:          mw.c,
			}

			matches, err := gsync.GetMatches(dPath.Path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error getting matches for '%s': %s", dPath.Path, err.Error())
				continue
			}

			if len(matches) == 0 && !isPatternLike(dPath.Path) {
				fmt.Fprintf(os.Stderr, "no file matched path '%s'\n", dPath.Path)
				continue
			}

			for _, match := range matches {
				out <- &DynamicPath{
					IsRemote:   true,
					IsForce:    dPath.IsForce,
					ServerName: dPath.ServerName,
					BaseAPIURL: dPath.BaseAPIURL,
					Path:       match,
				}
			}

		} else {
			matches, err := filepath.Glob(dPath.Path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "malformed pattern '%s': %s", dPath.Path, err.Error())
				continue
			}

			fileMatches := []string{}
			for _, match := range matches {
				stat, err := os.Stat(match)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error stating path '%s': %s", match, err.Error())
					continue
				}

				if !stat.IsDir() {
					fileMatches = append(fileMatches, match)
				}
			}

			if len(fileMatches) == 0 && !isPatternLike(dPath.Path) {
				fmt.Fprintf(os.Stderr, "no file or directory '%s'\n", dPath.Path)
				continue
			}

			for _, match := range matches {
				out <- &DynamicPath{
					IsRemote: false,
					IsForce:  dPath.IsForce,
					Path:     match,
				}
			}

		}
	}

}

func (mw *MassWriter) WriteToDest(srcs <-chan *DynamicPath, wg *sync.WaitGroup) {
	defer wg.Done()
	for src := range srcs {
		if src.IsRemote {
			gsync := client.GoSynClient{
				BaseAPIURL: src.BaseAPIURL,
				C:          mw.c,
			}

			reader, err := gsync.GetFile(src.Path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error getting file '%s' from url '%s': %s\n", src.Path, gsync.BaseAPIURL, err.Error())
				continue
			}

			if !mw.Dest.IsRemote {
				var destPath string
				if mw.DirMode {
					destPath = path.Join(mw.Dest.Path, path.Base(src.Path))
				} else {
					destPath = mw.Dest.Path
				}
				writer, err := os.Create(destPath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error creating file '%s': %s\n", destPath, err.Error())
					continue
				}

				if _, err = io.Copy(writer, reader); err != nil {
					fmt.Fprintf(os.Stderr, "error copying to '%s': %s\n", destPath, err.Error())
					continue
				}
			}

		} else {
			reader, err := os.Open(src.Path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error opening file '%s': %s\n", src.Path, err.Error())
				continue
			}

			if !mw.Dest.IsRemote {
				var destPath string
				if mw.DirMode {
					destPath = path.Join(mw.Dest.Path, path.Base(src.Path))
				} else {
					destPath = mw.Dest.Path
				}
				writer, err := os.Create(destPath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error creating file '%s': %s\n", destPath, err.Error())
					continue
				}

				if _, err = io.Copy(writer, reader); err != nil {
					fmt.Fprintf(os.Stderr, "error copying to '%s': %s\n", destPath, err.Error())
					continue
				}
			}

		}

	}
}

func isPatternLike(path string) bool {
	return strings.ContainsRune(path, '?') || strings.ContainsRune(path, '*')
}
