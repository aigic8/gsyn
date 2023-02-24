package main

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/aigic8/gosyn/api/client"
	u "github.com/aigic8/gosyn/cmd/gsyn/utils"
	"github.com/alexflint/go-arg"
	"github.com/fatih/color"
)

type (
	args struct {
		Cp *cpArgs `arg:"subcommand:cp"`
	}

	cpArgs struct {
		Force   bool     `arg:"-f"`
		Workers int      `arg:"-w,--workers"`
		Paths   []string `arg:"positional"`
		Timeout int64    `arg:"-t,--timeout"`
	}
)

const DEFAULT_TIMEOUT int64 = 5000

func main() {
	var args args
	arg.MustParse(&args)

	config, err := LoadConfig()
	if err != nil {
		errOut("loading configuration: %s", err.Error())
	}

	if args.Cp != nil {
		if config.Client == nil {
			errOut("no configuration found for client")
		}
		if args.Cp.Timeout == 0 {
			if config.Client.DefaultTimeout != 0 {
				args.Cp.Timeout = config.Client.DefaultTimeout
			} else {
				args.Cp.Timeout = DEFAULT_TIMEOUT
			}
		}
		CP(args.Cp, config.Client.Servers)
	}

}

func CP(cpArgs *cpArgs, servers map[string]string) {
	cwd, err := os.Getwd()
	if err != nil {
		errOut(err.Error())
	}
	c := &http.Client{Timeout: time.Duration(cpArgs.Timeout) * time.Millisecond}
	gc := &client.GoSynClient{C: c}

	pathsLen := len(cpArgs.Paths)
	if pathsLen < 2 {
		errOut("need at least a source and destination path")
	}

	srcs := make([]*u.DynamicPath, 0, pathsLen-1)
	for _, rawPath := range cpArgs.Paths[:pathsLen-1] {
		dPath, err := u.NewDynamicPath(rawPath, cwd, servers)
		if err != nil {
			errOut("malformed path: %s", err.Error())
		}
		srcs = append(srcs, dPath)
	}

	dest, err := u.NewDynamicPath(cpArgs.Paths[pathsLen-1], cwd, servers)
	if err != nil {
		errOut("malformed path: %s", err.Error())
	}

	// destDirMode is when we destinition MUST BE a directory to copy files to (when we have multiple sources or matches)
	destDirMode := len(srcs) > 1
	if destDirMode {
		stat, err := dest.Stat(gc)
		if err != nil {
			errOut(err.Error())
		}

		if !stat.IsDir {
			errOut("path '%s' is not a dir (multiple sources)", dest.String())
		}
	}

	// FIXME make multithreaded
	matches := []*u.DynamicPath{}
	for _, src := range srcs {
		srcMatches, err := src.GetMatches(gc)
		if err != nil {
			errOut("getting match for '%s': %s", src.String(), err.Error())
		}
		matches = append(matches, srcMatches...)
	}

	matchesLen := len(matches)
	if matchesLen == 0 {
		errOut("no file matched the sources")
		return
	}

	if !destDirMode && matchesLen > 1 {
		destDirMode = true
		stat, err := dest.Stat(gc)
		if err != nil {
			errOut("getting '%s' info: %s", dest.String(), err.Error())
		}

		if !stat.IsDir {
			errOut("path '%s' is not a dir (multiple sources)", dest.Path)
		}
	}

	// FIXME multithreaded
	for _, match := range matches {
		reader, err := match.Reader(gc)
		if err != nil {
			errOut("getting reader for '%s': %s", match.String(), err)
		}

		matchDest := dest
		if destDirMode {
			matchDest = &u.DynamicPath{IsRemote: dest.IsRemote, BaseAPIURL: dest.BaseAPIURL, ServerName: dest.ServerName, Path: path.Join(dest.Path, path.Base(match.Path))}
		}

		if err = matchDest.Copy(gc, path.Base(match.Path), cpArgs.Force, reader); err != nil {
			errOut("copying '%s' to '%s': %s", match.String(), matchDest.String(), err)
		}
	}

}

var errPrepend = color.New(color.FgRed).Sprint(" ERROR ")

func errOut(format string, a ...any) {
	fmt.Fprint(os.Stderr, errPrepend)
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}
