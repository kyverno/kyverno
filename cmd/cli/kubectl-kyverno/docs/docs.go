package docs

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
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

func Command(root *cobra.Command) *cobra.Command {
	var path string
	var website bool
	var autogenTag bool
	cmd := &cobra.Command{
		Use:     "docs",
		Short:   "Generates documentation.",
		Example: "",
		RunE: func(_ *cobra.Command, args []string) error {
			prepender := empty
			linkHandler := identity
			if website {
				prepender = websitePrepender
				linkHandler = websiteLinkHandler
			}
			if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
				if err := os.MkdirAll(path, os.ModeDir|os.ModePerm); err != nil {
					return err
				}
			}
			root.DisableAutoGenTag = !autogenTag
			return doc.GenMarkdownTreeCustom(root, path, prepender, linkHandler)
		},
	}
	cmd.Flags().StringVarP(&path, "output", "o", ".", "Output path")
	cmd.Flags().BoolVar(&website, "website", false, "Website version")
	cmd.Flags().BoolVar(&autogenTag, "autogenTag", true, "Determines if the generated docs should contain a timestamp")
	if err := cmd.MarkFlagDirname("output"); err != nil {
		log.Println("WARNING", err)
	}
	if err := cmd.MarkFlagRequired("output"); err != nil {
		log.Println("WARNING", err)
	}
	return cmd
}
