package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"

	"go.uber.org/multierr"
	"gopkg.in/yaml.v3"
)

type CommandExpectation struct {
	ExitCode *int
	StdOut   *string
	StdErr   *string
}

func (x CommandExpectation) Verify(stdout []byte, stderr []byte, err error) error {
	exitcode := 0
	if err != nil {
		exitError := err.(*exec.ExitError)
		exitcode = exitError.ExitCode()
	}
	if x.ExitCode != nil {
		if exitcode != *x.ExitCode {
			return errors.New(fmt.Sprint("unexpected exit code, expected: ", *x.ExitCode, " - actual: ", exitcode))
		}
	}
	if x.StdOut != nil {
		if trim(*x.StdOut, "\n", " ") != trim(string(stdout), "\n", " ") {
			return errors.New(fmt.Sprint("unexpected stdout, expected: ", *x.StdOut, " - actual: ", string(stdout)))
		}
	}
	if x.StdErr != nil {
		if trim(*x.StdErr, "\n", " ") != trim(string(stderr), "\n", " ") {
			return errors.New(fmt.Sprint("unexpected stderr, expected: ", *x.StdErr, " - actual: ", string(stderr)))
		}
	}
	return nil
}

type KubectlTest struct {
	Args   []string
	Expect *CommandExpectation
}

func (kt KubectlTest) Run(name string) error {
	stdout, stderr, err := runCommand("kubectl", kt.Args...)
	if kt.Expect != nil {
		return kt.Expect.Verify(stdout, stderr, err)
	}
	return nil
}

type Test struct {
	Kubectl *KubectlTest
}

func (t Test) Run(name string) error {
	if t.Kubectl != nil {
		return t.Kubectl.Run(name)
	}
	return errors.New("no test defined")
}

func trim(in string, s ...string) string {
	for _, s := range s {
		in = strings.TrimSuffix(in, s)
	}
	return in
}

func runCommand(name string, arg ...string) ([]byte, []byte, error) {
	cmd := exec.Command(name, arg...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func stdCommand(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func makeCluster() error {
	cmd := stdCommand("make", "kind-create-cluster", "kind-deploy-kyverno")
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func makeDeleteCluster() error {
	cmd := stdCommand("make", "kind-delete-cluster")
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func loadTests() (map[string][]Test, error) {
	data, err := ioutil.ReadFile("./test/conformance/tests.yaml")
	if err != nil {
		return nil, err
	}
	tests := map[string][]Test{}
	if err := yaml.Unmarshal(data, tests); err != nil {
		return nil, err
	}
	return tests, nil
}

func main() {
	tests, err := loadTests()
	if err != nil {
		log.Fatal(err)
	}
	for cluster, tests := range tests {
		runner := func(name string, tests []Test) error {
			os.Setenv("KIND_NAME", name)
			defer func(name string) {
				if err := makeDeleteCluster(); err != nil {
					log.Fatal(err)
				}
			}(name)
			if err := makeCluster(); err != nil {
				return err
			}
			var errs []error
			for _, test := range tests {
				if err := test.Run(name); err != nil {
					errs = append(errs, err)
				}
			}
			return multierr.Combine(errs...)
		}
		if err := runner(cluster, tests); err != nil {
			log.Fatal(err)
		}
	}
}
