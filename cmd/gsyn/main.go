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
	ServerName string
	Path       string
}

func main() {
	var args args
	arg.MustParse(&args)
	// TODO
	servers := map[string]string{}

	if args.Cp != nil {
		pathsLen := len(args.Cp.Paths)
		if pathsLen < 2 {
			fmt.Fprintln(os.Stderr, "need at least a source and destination path")
			os.Exit(1)
		}

		srcs := make([]DynamicPath, 0, pathsLen-1)
		for _, rawPath := range args.Cp.Paths[:pathsLen-1] {
			dPath, err := ParseDynamicPath(rawPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "malformed path: %v\n", err)
				os.Exit(1)
			}
			srcs = append(srcs, dPath)
			// FIXME validate paths
		}

		// TODO for now dest only can be local
		dest, err := ParseDynamicPath(args.Cp.Paths[pathsLen-1])
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

		t := NewMassWriter(servers, time.Duration(args.Cp.Timeout)*time.Millisecond, &dest, destDirMode)

		srcsChan := make(chan *DynamicPath)
		matchesChan := make(chan *Match)

		matchesWg := new(sync.WaitGroup)
		matchesWg.Add(args.Cp.Workers)
		for i := 0; i < args.Cp.Workers; i++ {
			go t.GetMatches(srcsChan, matchesChan, matchesWg)
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
			t.WriteToDest(matchesChan, writesWg)
		}

		writesWg.Wait()
	}

}

func ParseDynamicPath(rawPath string) (DynamicPath, error) {
	pathParts := strings.Split(rawPath, ":")
	pathPartsLen := len(pathParts)
	if pathPartsLen > 2 {
		return DynamicPath{}, errors.New("more than one collon")
	}
	if pathPartsLen == 1 {
		return DynamicPath{
			IsRemote: false,
			Path:     pathParts[0],
		}, nil
	}

	// pathPartsLen == 2
	return DynamicPath{
		IsRemote:   true,
		ServerName: pathParts[0],
		Path:       pathParts[1],
	}, nil
}

type MassWriter struct {
	c       *http.Client
	Servers map[string]string
	Dest    *DynamicPath
	DirMode bool
}

func NewMassWriter(servers map[string]string, timeout time.Duration, dest *DynamicPath, dirMode bool) *MassWriter {
	return &MassWriter{
		Servers: servers,
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

func (t *MassWriter) GetMatches(dPaths <-chan *DynamicPath, out chan<- *Match, wg *sync.WaitGroup) {
	defer wg.Done()
	for dPath := range dPaths {
		if dPath.IsRemote {
			baseAPIURL, serverExists := t.Servers[dPath.ServerName]
			if !serverExists {
				fmt.Fprintf(os.Stderr, "server with name '%s' does not exist", dPath.ServerName)
				return
			}

			gsync := client.GoSynClient{
				BaseAPIURL: baseAPIURL,
				C:          t.c,
			}

			matches, err := gsync.GetMatches(dPath.Path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error getting matches for '%s': %s", dPath.Path, err.Error())
				return
			}

			out <- &Match{
				IsRemote:   true,
				BaseAPIURL: baseAPIURL,
				Matches:    matches,
			}
		} else {
			matches, err := filepath.Glob(dPath.Path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "malformed pattern '%s': %s", dPath.Path, err.Error())
				return
			}

			fileMatches := []string{}
			for _, match := range matches {
				stat, err := os.Stat(match)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error stating path '%s': %s", match, err.Error())
				}

				if !stat.IsDir() {
					fileMatches = append(fileMatches, match)
				}
			}

			out <- &Match{
				IsRemote: false,
				Matches:  fileMatches,
			}
		}
	}

}

// TODO instead of each match having an array, it should have just one match
// this way the work can better spread between workers
func (t *MassWriter) WriteToDest(matches <-chan *Match, wg *sync.WaitGroup) {
	defer wg.Done()
	for match := range matches {
		if match.IsRemote {
			gsync := client.GoSynClient{
				BaseAPIURL: match.BaseAPIURL,
				C:          t.c,
			}

			for _, filePath := range match.Matches {
				reader, err := gsync.GetFile(filePath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error getting file '%s' from url '%s': %s\n", filePath, gsync.BaseAPIURL, err.Error())
					continue
				}

				if !t.Dest.IsRemote {
					var destPath string
					if t.DirMode {
						destPath = path.Join(t.Dest.Path, path.Base(filePath))
					} else {
						destPath = t.Dest.Path
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
		} else {
			for _, filePath := range match.Matches {
				reader, err := os.Open(filePath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error opening file '%s': %s\n", filePath, err.Error())
					continue
				}

				if !t.Dest.IsRemote {
					var destPath string
					if t.DirMode {
						destPath = path.Join(t.Dest.Path, path.Base(filePath))
					} else {
						destPath = t.Dest.Path
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
}
