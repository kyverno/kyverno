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

const fmTemplateNoDate = `---
title: "%s"
weight: 35
---
`

func websitePrepender(noDate bool) func(string) string {
	now := time.Now().Format(time.RFC3339)
	return func(filename string) string {
		name := filepath.Base(filename)
		base := strings.TrimSuffix(name, path.Ext(name))
		if !noDate {
			return fmt.Sprintf(fmTemplate, now, strings.Replace(base, "_", " ", -1))
		}
		return fmt.Sprintf(fmTemplateNoDate, strings.Replace(base, "_", " ", -1))
	}
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
