package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path"
	"sync"
	"time"

	"github.com/aigic8/gosyn/api"
	"github.com/aigic8/gosyn/api/client"
	apiUtils "github.com/aigic8/gosyn/api/handlers/utils"
	"github.com/aigic8/gosyn/cmd/gsyn/config"
	u "github.com/aigic8/gosyn/cmd/gsyn/utils"
	"github.com/alexflint/go-arg"
	"github.com/fatih/color"
	"github.com/quic-go/quic-go/http3"
	"github.com/schollz/progressbar/v3"
)

type (
	args struct {
		Cp    *cpArgs    `arg:"subcommand:cp"`
		Serve *serveArgs `arg:"subcommand:serve"`
	}

	cpArgs struct {
		Config  string   `arg:"-c,--config"`
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
const DEFAULT_WORKERS int = 10

func main() {
	var args args
	arg.MustParse(&args)
	go signalHandler()

	var configPaths []string
	var err error
	if args.Serve != nil && args.Serve.Config != "" {
		configPaths = []string{args.Serve.Config}
	} else if args.Cp != nil && args.Cp.Config != "" {
		configPaths = []string{args.Cp.Config}
	} else {
		configPaths, err = config.GetConfigPaths()
		if err != nil {
			errOut("getting configuration paths: %s", err.Error())
		}
	}

	config, err := config.LoadConfig(configPaths)
	if err != nil {
		errOut("loading configuration: %s", err.Error())
	}

	// FIXME maybe load all the certificates to memory all at once in init?
	serverInfos := map[string]*u.ServerInfo{}
	if config.Client != nil {
		for serverName, info := range config.Client.Servers {
			serverInfos[serverName] = &u.ServerInfo{
				Name:         serverName,
				BaseAPIURL:   info.Address,
				GUID:         info.GUID,
				Certificates: info.Certificates,
			}
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

		if args.Cp.Workers == 0 {
			if config.Client.DefaultWorkers != 0 {
				args.Cp.Workers = config.Client.DefaultWorkers
			} else {
				args.Cp.Workers = DEFAULT_WORKERS
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

func signalHandler() {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel)
	for {
		s := <-signalChannel
		if s == os.Interrupt || s == os.Kill {
			os.Exit(0)
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

	// destDirMode is when we destination MUST BE a directory to copy files to (when we have multiple sources or matches)
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

	matches := []*u.DynamicPath{}
	srcsChann := make(chan *u.DynamicPath, cpArgs.Workers)
	matchesChann := make(chan *u.DynamicPath, cpArgs.Workers)
	wg := new(sync.WaitGroup)
	wg.Add(cpArgs.Workers)

	for i := 0; i < cpArgs.Workers; i++ {
		go getMatchesAsync(gc, srcsChann, matchesChann, wg)
	}

	go func() {
		defer close(matchesChann)

		for _, src := range srcs {
			srcsChann <- src
		}
		close(srcsChann)

		wg.Wait()
	}()

	for match := range matchesChann {
		matches = append(matches, match)
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

	matchesOutChann := make(chan *u.DynamicPath, cpArgs.Workers)
	cpWg := new(sync.WaitGroup)
	cpWg.Add(cpArgs.Workers)

	for i := 0; i < cpArgs.Workers; i++ {
		go copyAsync(gc, matchesOutChann, dest, destDirMode, cpArgs.Force, cpWg)
	}

	go func() {
		defer close(matchesOutChann)
		for _, match := range matches {
			matchesOutChann <- match
		}
	}()

	cpWg.Wait()
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

func getMatchesAsync(gc *client.GoSynClient, srcs <-chan *u.DynamicPath, out chan<- *u.DynamicPath, wg *sync.WaitGroup) {
	defer wg.Done()
	for src := range srcs {
		matches, err := src.GetMatches(gc)
		if err != nil {
			warn("getting match for '%s': %s", src.String(), err.Error())
		}

		if len(matches) != 0 {
			for _, match := range matches {
				out <- match
			}
		}
	}
}

func copyAsync(gc *client.GoSynClient, matches <-chan *u.DynamicPath, dest *u.DynamicPath, destDirMode bool, force bool, wg *sync.WaitGroup) {
	defer wg.Done()
	for match := range matches {
		reader, size, err := match.Reader(gc)
		if err != nil {
			errOut("reading '%s': %s", match.String(), err)
		}

		bar := progressbar.NewOptions64(
			size,
			progressbar.OptionSetDescription(match.String()),
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionShowBytes(true),
			progressbar.OptionSetWidth(20),
			progressbar.OptionThrottle(65*time.Millisecond),
			progressbar.OptionShowCount(),
			progressbar.OptionOnCompletion(func() {
				fmt.Fprint(os.Stderr, "\n")
			}),
			progressbar.OptionSpinnerType(14),
			progressbar.OptionSetRenderBlankState(false),
		)
		r := io.TeeReader(reader, bar)

		matchDest := dest
		if destDirMode {
			matchDest = &u.DynamicPath{IsRemote: dest.IsRemote, Server: dest.Server, Path: path.Join(dest.Path, path.Base(match.Path))}
		}

		if err = matchDest.Copy(gc, path.Base(match.Path), force, r); err != nil {
			errOut("copying '%s' to '%s': %s", match.String(), matchDest.String(), err)
		}
		reader.Close()
	}
}
