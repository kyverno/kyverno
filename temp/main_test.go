package main

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

var image = "nginx:latest"

func Benchmark_Remote(b *testing.B) {
	for i := 0; i < b.N; i++ {
		useremote(image)
	}
}

func Benchmark_Crane(b *testing.B) {
	for i := 0; i < b.N; i++ {
		usecrane(image)
	}
}

func usecrane(image string) {
	_, err := crane.Manifest(image)
	if err != nil {
		panic(err)
	}

	_, err = crane.Config(image)
	if err != nil {
		panic(err)
	}

	_, err = crane.Get(image)
	if err != nil {
		panic(err)
	}
}

func useremote(image string) {
	ref, err := name.ParseReference(image)
	if err != nil {
		panic(err)
	}

	img, err := remote.Image(ref)
	if err != nil {
		panic(err)
	}

	_, err = img.Manifest()
	if err != nil {
		panic(err)
	}

	_, err = img.ConfigFile()
	if err != nil {
		panic(err)
	}

	_, err = img.Digest()
	if err != nil {
		panic(err)
	}
}
