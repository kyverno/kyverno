package main

import (
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func getManifests(folder string) ([]fs.FileInfo, error) {
	return ioutil.ReadDir(folder)
}

func createCommand(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	// cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func makeCluster() error {
	cmd := createCommand("make", "kind-create-cluster", "kind-deploy-kyverno")
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func makeDeleteCluster() error {
	cmd := createCommand("make", "kind-delete-cluster")
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func kubectl(file string) error {
	cmd := createCommand("kubectl", "create", "-f", file)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func main() {
	runner := func(index int, path string, file fs.FileInfo) error {
		name := fmt.Sprint("test-", index)
		os.Setenv("KIND_NAME", name)
		defer func(name string) {
			if err := makeDeleteCluster(); err != nil {
				log.Fatal(err)
			}
		}(name)
		if err := makeCluster(); err != nil {
			return err
		}
		if err := kubectl(filepath.Join(path, file.Name())); err != nil {
			return err
		}
		return nil
	}
	shouldFailPath := "./test/conformance/manifests/should-fail"
	files, err := getManifests(shouldFailPath)
	if err != nil {
		log.Fatal(err)
	}
	for i, file := range files {
		if err := runner(i, shouldFailPath, file); err == nil {
			log.Fatal(errors.New(fmt.Sprint("no error returned but one was expected when applying manifest", file.Name())))
		}
	}
}
