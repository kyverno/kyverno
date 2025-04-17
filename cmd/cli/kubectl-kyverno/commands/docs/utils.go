package docs

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const fmTemplate = `---
date: %s
title: "%s"
weight: 35
---
`

func websitePrepender(filename string) string {
	now := time.Now().Format(time.RFC3339)
	name := filepath.Base(filename)
	base := strings.TrimSuffix(name, path.Ext(name))
	return fmt.Sprintf(fmTemplate, now, strings.Replace(base, "_", " ", -1))
}

func websiteLinkHandler(filename string) string {
	return "../" + strings.TrimSuffix(filename, filepath.Ext(filename))
}

func identity(s string) string {
	return s
}

func empty(s string) string {
	return ""
}
