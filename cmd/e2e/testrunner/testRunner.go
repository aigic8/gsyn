package testrunner

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/aigic8/gosyn/cmd/e2e/utils"
	"github.com/aigic8/gosyn/cmd/gsyn/config"
)

type TestRunner struct {
	users   map[string]utils.TestFileUserData
	servers map[string]utils.TestFileServerData
	spaces  map[string]string
	cert    utils.TestFileCertData
	tempDir string
	cwd     string
}

func NewTestRunner(testsData *utils.TestFileData) (*TestRunner, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	usersMap, serversMap, err := utils.MakeUsersAndServersMap(testsData.Users, testsData.Servers)
	if err != nil {
		return nil, fmt.Errorf("making users and servers map: %w", err)
	}
	if len(usersMap) == 0 {
		fmt.Fprintln(os.Stderr, "warn: no users was found")
	}
	if len(serversMap) == 0 {
		fmt.Fprintln(os.Stderr, "warn: no servers was found")
	}

	tempDir, err := os.MkdirTemp(os.TempDir(), "*")
	if err != nil {
		return nil, fmt.Errorf("creating tempdir: %w", err)
	}

	return &TestRunner{
		users:   usersMap,
		servers: serversMap,
		spaces:  testsData.Spaces,
		cert:    testsData.Cert,
		tempDir: tempDir,
		cwd:     cwd,
	}, nil
}

func (tr *TestRunner) Run(test *utils.TestFileTestData) {
	user, userExists := tr.users[test.User]
	if !userExists {
		fmt.Fprintf(os.Stderr, "%s: FAILD - error: user with name '%s' does not exist\n", test.Name, test.User)
		return
	}

	server, serverExists := tr.servers[test.Server]
	if !serverExists {
		fmt.Fprintf(os.Stderr, "%s: FAILED - error: server with name '%s' does not exist\n", test.Name, test.Server)
		return
	}

	spaces, err := tr.makeSpaces(server, test.MakeSpacesDirs)
	if test.MakeSpacesDirs {
		defer cleanPaths(mapValues(spaces)...)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: FAILED - error: making spaces: %v\n", test.Name, err)
		return
	}

	madeFiles, err := tr.makeFiles(test.MakeFiles)
	defer cleanPaths(madeFiles...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: FAILED - error: making files: %v\n", test.Name, err)
		return
	}

	configInfo, err := utils.GenerateConfigs(utils.ConfigOptions{
		Users: []config.ServerUser{{GUID: user.UUID, Spaces: server.Users[0].Spaces}}, // FIXME: fix only the first user
		Servers: map[string]config.ClientServerItem{
			server.Name: {
				GUID:         user.UUID,
				Address:      server.Address,
				Certificates: []string{tr.cert.Der},
			},
		},
		Spaces:     spaces,
		PrivKeyPem: path.Join(tr.cwd, tr.cert.Key),
		CertPem:    path.Join(tr.cwd, tr.cert.Pem),
		CertDer:    path.Join(tr.cwd, tr.cert.Der),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: FAILED: error: generating config files: %v\n", test.Name, err)
		return
	}
	defer func() {
		if err := configInfo.Clean(); err != nil {
			// TODO: write a wrapper around fmt.Fprintf for errors
			fmt.Fprintf(os.Stderr, "%s: error: cleaning config files: %v\n", test.Name, err)
		}
	}()

	doneChannel := make(chan bool)
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go tr.runServer(test.Name, configInfo.ServerConfigPath, doneChannel, wg)
	defer func() {
		doneChannel <- true
		wg.Wait()
	}()
	time.Sleep(100 * time.Millisecond) // wait for the server to spin up

	err = tr.runClientCommands(configInfo.ClientConfigPath, test.Commands)
	if err != nil {
		if !test.ExpectError {
			fmt.Fprintf(os.Stderr, "%s: FAILED - error: running command: %v\n", test.Name, err)
		} else {
			fmt.Fprintf(os.Stderr, "%s: SUCCESS\n", test.Name)
		}
		return
	}
	if test.ExpectError {
		// TODO: replace Fprintf with Printf in the right context
		fmt.Fprintf(os.Stderr, "%s: FAILED: error: expected errors\n", test.Name)
		return
	}

	expectedFiles, err := tr.checkExpectedFiles(test.ExpectFiles)
	defer cleanPaths(expectedFiles...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: FAILED - %v\n", test.Name, err)
		return
	}

	// TODO: instead of each function replacing variables, one function should preprocess and replace them
	expectedDirs, err := tr.checkExpectedDirs(test.ExpectDirs)
	defer cleanPaths(expectedDirs...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: FAILED - %v\n", test.Name, err)
		return
	}

	fmt.Fprintf(os.Stderr, "%s: SUCCESS\n", test.Name)
}

// returns server spaces map replacing the variables like $TMP, also create their directory if makeSpacesDir is true. Don't forget to defer cleanPaths if you set makeSpacesDir true.
func (tr *TestRunner) makeSpaces(server utils.TestFileServerData, makeSpacesDirs bool) (map[string]string, error) {
	spaces := map[string]string{}
	for _, user := range server.Users {
		for _, spaceName := range user.Spaces {
			spacePathRaw, spaceExists := tr.spaces[spaceName]
			if !spaceExists {
				return spaces, fmt.Errorf("space with name '%s' does not exist", spaceName)
			}
			spacePath := strings.ReplaceAll(spacePathRaw, "$TMP", tr.tempDir)
			if makeSpacesDirs {
				if err := os.Mkdir(spacePath, 0777); err != nil {
					return spaces, fmt.Errorf("making space dir '%s': %w", spacePath, err)
				}
			}
			spaces[spaceName] = spacePath
		}
	}

	return spaces, nil
}

// creates and returns test files replacing the variables like $TMP. Don't forget to defer cleanPaths.
func (tr *TestRunner) makeFiles(makeFiles []string) ([]string, error) {
	res := []string{}
	for _, filePathRaw := range makeFiles {
		filePath := strings.ReplaceAll(filePathRaw, "$TMP", tr.tempDir)
		res = append(res, filePath)
		if err := os.WriteFile(filePath, []byte{}, 0666); err != nil {
			return res, fmt.Errorf("making file '%s': %w\n", filePath, err)
		}
	}
	return res, nil
}

func (tr *TestRunner) runServer(testName string, serverConfigPath string, doneChannel <-chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	serveCommand := exec.Command(path.Join(tr.cwd, "gsyn"), "serve", "-c", serverConfigPath)
	isDone := false
	m := new(sync.Mutex)
	wg.Add(1)
	go func(isDone *bool, m *sync.Mutex, wg *sync.WaitGroup) {
		defer wg.Done()
		output, err := serveCommand.CombinedOutput()
		m.Lock()
		defer m.Unlock()
		if !*isDone && err != nil {
			fmt.Fprintf(os.Stderr, "%s: FAILED: error: running serve command: %s\n", testName, string(output))
			return
		}
	}(&isDone, m, wg)
	<-doneChannel
	m.Lock()
	if err := serveCommand.Process.Signal(os.Interrupt); err != nil {
		fmt.Fprintf(os.Stderr, "%s: error: sending Interrupt signal to send command: %v\n", testName, err)
	}
	isDone = true
	m.Unlock()
}

func (tr *TestRunner) runClientCommands(clientConfigPath string, commands []string) error {
	// TODO: don't hardcode exec path, get it from outside
	client := utils.Client{ExecPath: path.Join(tr.cwd, "gsyn"), ConfigPath: clientConfigPath}
	for _, commandRaw := range commands {
		command := strings.ReplaceAll(commandRaw, "$TMP", tr.tempDir)
		if err := client.Run(command); err != nil {
			return fmt.Errorf("command %s: %v", command, err)
		}
	}
	return nil
}

func (tr *TestRunner) checkExpectedFiles(expectedFiles []string) ([]string, error) {
	res := []string{}
	for _, expectedFileRaw := range expectedFiles {
		expectedFile := strings.ReplaceAll(expectedFileRaw, "$TMP", tr.tempDir)
		res = append(res, expectedFile)
		if err := utils.IsPathFile(expectedFile); err != nil {
			return res, fmt.Errorf("was expecting file '%s': %v\n", expectedFile, err)
		}
	}
	return res, nil
}

func (tr *TestRunner) checkExpectedDirs(expectedDirs []string) ([]string, error) {
	res := []string{}
	for _, expectedDirRaw := range expectedDirs {
		expectedDir := strings.ReplaceAll(expectedDirRaw, "$TMP", tr.tempDir)
		res = append(res, expectedDir)
		if err := utils.IsPathDir(expectedDir); err != nil {
			return res, fmt.Errorf("was expecting dir '%s': %v\n", expectedDir, err)
		}
	}
	return res, nil
}

func cleanPaths(paths ...string) {
	for _, path := range paths {
		os.RemoveAll(path)
	}
}

func mapValues(input map[string]string) []string {
	res := []string{}
	for _, value := range input {
		res = append(res, value)
	}
	return res
}
