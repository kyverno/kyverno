package data

import (
	"embed"
	"io/fs"
)

const crdsFolder = "crds"

//go:embed crds
var crdsFs embed.FS

func Crds() (fs.FS, error) {
	return fs.Sub(crdsFs, crdsFolder)
}
