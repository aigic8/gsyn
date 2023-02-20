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

func main() {
	var args args
	arg.MustParse(&args)

	if args.Cp != nil {
		CP(args.Cp)
	}

}

func CP(cpArgs *cpArgs) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	c := &http.Client{Timeout: time.Duration(cpArgs.Timeout) * time.Millisecond}
	gc := &client.GoSynClient{C: c}

	// TODO
	servers := map[string]string{}

	pathsLen := len(cpArgs.Paths)
	if pathsLen < 2 {
		fmt.Fprintln(os.Stderr, "need at least a source and destination path")
		os.Exit(1)
	}

	srcs := make([]*u.DynamicPath, 0, pathsLen-1)
	for _, rawPath := range cpArgs.Paths[:pathsLen-1] {
		dPath, err := u.NewDynamicPath(rawPath, cwd, servers)
		if err != nil {
			fmt.Fprintf(os.Stderr, "malformed path: %s\n", err.Error())
			os.Exit(1)
		}
		srcs = append(srcs, dPath)
	}

	dest, err := u.NewDynamicPath(cpArgs.Paths[pathsLen-1], cwd, servers)
	if err != nil {
		fmt.Fprintf(os.Stderr, "malformed path: %s\n", err.Error())
		os.Exit(1)
	}

	// destDirMode is when we destinition MUST BE a directory to copy files to (when we have multiple sources or matches)
	destDirMode := len(srcs) > 1
	if destDirMode {
		stat, err := dest.Stat(gc)
		if err != nil {
			panic(err)
		}

		if !stat.IsDir {
			panic("path '%s' is not a dir (multiple sources)")
		}
	}

	// FIXME make multithreaded
	matches := []*u.DynamicPath{}
	for _, src := range srcs {
		srcMatches, err := src.GetMatches(gc)
		if err != nil {
			panic(err)
		}
		matches = append(matches, srcMatches...)
	}

	matchesLen := len(matches)
	if matchesLen == 0 {
		fmt.Fprintln(os.Stderr, "no file matched the sources")
		return
	}

	if !destDirMode && matchesLen > 1 {
		destDirMode = true
		stat, err := dest.Stat(gc)
		if err != nil {
			panic(err)
		}

		if !stat.IsDir {
			panic("path '%s' is not a dir (multiple sources)")
		}
	}

	// FIXME multithreaded
	for _, match := range matches {
		reader, err := match.Reader(gc)
		if err != nil {
			panic(err)
		}

		matchDest := dest
		if destDirMode {
			matchDest = &u.DynamicPath{IsRemote: dest.IsRemote, BaseAPIURL: dest.BaseAPIURL, ServerName: dest.ServerName, Path: path.Join(dest.Path, path.Base(match.Path))}
		}

		if err = matchDest.Copy(gc, path.Base(match.Path), cpArgs.Force, reader); err != nil {
			panic(err)
		}
	}

}
