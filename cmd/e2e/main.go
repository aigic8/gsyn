package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/aigic8/gosyn/cmd/e2e/testrunner"
	"github.com/aigic8/gosyn/cmd/e2e/utils"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	testsData, err := utils.ParseTestFile(path.Join(cwd, "tests.toml"))
	if err != nil {
		panic(err)
	}

	buildPath := path.Join(cwd, "../gsyn")
	execPath := path.Join(cwd, "gsyn")
	mustRunCommand("go", "build", buildPath)
	defer mustRemovePath(execPath)

	tr, err := testrunner.NewTestRunner(testsData)
	if err != nil {
		panic(err)
	}

	for _, test := range testsData.Tests {
		tr.Run(&test)
	}
}

func mustRunCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Errorf("error running command: %s", string(output)))
	}
	return cmd
}

func mustRemovePath(path string) {
	if err := os.Remove(path); err != nil {
		panic(err)
	}
}
