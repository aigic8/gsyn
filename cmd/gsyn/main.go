package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/aigic8/gosyn/api"
	"github.com/aigic8/gosyn/api/client"
	apiUtils "github.com/aigic8/gosyn/api/handlers/utils"
	u "github.com/aigic8/gosyn/cmd/gsyn/utils"
	"github.com/alexflint/go-arg"
	"github.com/fatih/color"
	"github.com/quic-go/quic-go/http3"
)

type (
	args struct {
		Cp    *cpArgs    `arg:"subcommand:cp"`
		Serve *serveArgs `arg:"subcommand:serve"`
	}

	cpArgs struct {
		Force   bool     `arg:"-f"`
		Workers int      `arg:"-w,--workers"`
		Paths   []string `arg:"positional"`
		Timeout int64    `arg:"-t,--timeout"`
	}

	serveArgs struct {
		Config string `arg:"-c,--config"`
	}
)

const DEFAULT_TIMEOUT int64 = 5000

func main() {
	var args args
	arg.MustParse(&args)

	var configPaths []string
	var err error
	if args.Serve != nil && args.Serve.Config != "" {
		configPaths = []string{args.Serve.Config}
	} else {
		configPaths, err = getConfigPaths()
		if err != nil {
			errOut("getting configuration paths: %s", err.Error())
		}
	}

	config, err := LoadConfig(configPaths)
	if err != nil {
		errOut("loading configuration: %s", err.Error())
	}

	// FIXME maybe load all the certificates to memory all at once in init?
	serverInfos := map[string]*u.ServerInfo{}
	for serverName, info := range config.Client.Servers {
		serverInfos[serverName] = &u.ServerInfo{
			Name:         serverName,
			BaseAPIURL:   info.Address,
			GUID:         info.GUID,
			Certificates: info.Certificates,
		}
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
		CP(args.Cp, serverInfos)

	} else if args.Serve != nil {
		if config.Server == nil {
			errOut("no configuration found for server")
		}

		for _, spacePath := range config.Server.Spaces {
			if err = validateSpacePath(spacePath); err != nil {
				warn("validating space '%s': %s", spacePath, err.Error())
			}
		}

		// FIXME bad dependency apiUtils, find a way to resolve
		users := map[string]apiUtils.UserInfo{}
		if config.Server.Users == nil || len(config.Server.Users) == 0 {
			warn("starting server with no users!")
		} else {
			for _, user := range config.Server.Users {
				spacesMap := map[string]bool{}
				for _, space := range user.Spaces {
					if _, ok := config.Server.Spaces[space]; !ok {
						errOut("unknown space '%s'", space)
					}
					spacesMap[space] = true
				}
				users[user.GUID] = apiUtils.UserInfo{GUID: user.GUID, Spaces: spacesMap}
			}
		}

		r := api.Router(config.Server.Spaces, users)
		err = api.Serve(r, config.Server.Address, config.Server.CertPath, config.Server.PrivPath)
		if err != nil {
			errOut("running server: %s", err.Error())
		}
	}

}

func CP(cpArgs *cpArgs, servers map[string]*u.ServerInfo) {
	cwd, err := os.Getwd()
	if err != nil {
		errOut(err.Error())
	}

	pathsLen := len(cpArgs.Paths)
	if pathsLen < 2 {
		errOut("need at least a source and destination path")
	}

	certs := map[string]bool{}
	srcs := make([]*u.DynamicPath, 0, pathsLen-1)
	for _, rawPath := range cpArgs.Paths[:pathsLen-1] {
		dPath, err := u.NewDynamicPath(rawPath, cwd, servers)
		if err != nil {
			errOut("malformed path: %s", err.Error())
		}
		srcs = append(srcs, dPath)

		if dPath.IsRemote {
			for _, cert := range dPath.Server.Certificates {
				certs[cert] = true
			}
		}
	}

	dest, err := u.NewDynamicPath(cpArgs.Paths[pathsLen-1], cwd, servers)
	if err != nil {
		errOut("malformed path: %s", err.Error())
	}
	if dest.IsRemote {
		for _, cert := range dest.Server.Certificates {
			certs[cert] = true
		}
	}

	tlsConfig, err := makeTLSConfig(certs)
	if err != nil {
		errOut("configuring TLS: %s", err.Error())
	}

	c := &http.Client{
		Timeout:   time.Duration(cpArgs.Timeout) * time.Millisecond,
		Transport: &http3.RoundTripper{TLSClientConfig: tlsConfig},
	}

	gc := &client.GoSynClient{C: c}

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
			matchDest = &u.DynamicPath{IsRemote: dest.IsRemote, Server: dest.Server, Path: path.Join(dest.Path, path.Base(match.Path))}
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

var warnPrepend = color.New(color.FgYellow).Sprint(" WARN ")

func warn(format string, a ...any) {
	fmt.Fprint(os.Stderr, warnPrepend)
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}

func validateSpacePath(spacePath string) error {
	stat, err := os.Stat(spacePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("path does not exist")
		}
		return err
	}

	if !stat.IsDir() {
		return errors.New("path is not a directory")
	}

	return nil
}

func makeTLSConfig(certificatePaths map[string]bool) (*tls.Config, error) {
	if len(certificatePaths) == 0 {
		return nil, nil
	}

	certPool := x509.NewCertPool()
	for certPath := range certificatePaths {
		certFile, err := os.Open(certPath)
		if err != nil {
			return nil, err
		}
		defer certFile.Close()

		certBytes, err := io.ReadAll(certFile)
		if err != nil {
			return nil, err
		}

		cert, err := x509.ParseCertificate(certBytes)
		if err != nil {
			return nil, err
		}

		certPool.AddCert(cert)
	}

	return &tls.Config{
		RootCAs: certPool,
	}, nil
}
