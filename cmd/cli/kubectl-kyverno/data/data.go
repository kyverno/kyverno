package data

import (
	"embed"
	"io/fs"
)

const CrdsFolder = "crds"

//go:embed crds
var crdsFs embed.FS

func Crds() fs.FS {
	return crdsFs
}
