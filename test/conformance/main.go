package main

import (
	"bytes"
	"errors"
	"flag"
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
			return errors.New(fmt.Sprint("unexpected exit code\n  expected: ", *x.ExitCode, "\n  actual:   ", exitcode))
		}
	}
	if x.StdOut != nil {
		if trim(*x.StdOut, "\n", " ") != trim(string(stdout), "\n", " ") {
			return errors.New(fmt.Sprint("unexpected stdout\n  expected: ", *x.StdOut, "\n  actual:   ", string(stdout)))
		}
	}
	if x.StdErr != nil {
		if trim(*x.StdErr, "\n", " ") != trim(string(stderr), "\n", " ") {
			return errors.New(fmt.Sprint("unexpected stderr\n  expected: ", *x.StdErr, "\n  actual:   ", string(stderr)))
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
		if err := kt.Expect.Verify(stdout, stderr, err); err != nil {
			log.Println("--- STDERR ---")
			log.Println(string(stderr))
			log.Println("--- STDOUT ---")
			log.Println(string(stdout))
			return err
		}
	}
	return nil
}

type Test struct {
	Description string
	Kubectl     *KubectlTest
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
	var createCluster bool
	var deleteCluster bool
	flag.BoolVar(&createCluster, "create-cluster", true, "Set this flag to 'false', to use an existing cluster.")
	flag.BoolVar(&deleteCluster, "delete-cluster", true, "Set this flag to 'false', to not delete the created cluster.")
	flag.Parse()

	tests, err := loadTests()
	if err != nil {
		log.Fatal(err)
	}
	for cluster, tests := range tests {
		runner := func(name string, tests []Test) error {
			if err := os.Setenv("KIND_NAME", name); err != nil {
				return err
			}
			if createCluster {
				if err := makeCluster(); err != nil {
					return err
				}
				if deleteCluster {
					defer func(name string) {
						if err := makeDeleteCluster(); err != nil {
							log.Fatal(err)
						}
					}(name)
				}
			}
			var errs []error
			for _, test := range tests {
				log.Println("Running test", test.Description, "...")
				if err := test.Run(name); err != nil {
					log.Println("FAILED: ", err)
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
