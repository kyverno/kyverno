package docs

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

type options struct {
	path       string
	website    bool
	autogenTag bool
}

func (o options) validate(root *cobra.Command) error {
	if o.path == "" {
		return errors.New("path is required")
	}
	if root == nil {
		return errors.New("root command is required")
	}
	return nil
}

func (o options) execute(root *cobra.Command) error {
	prepender := empty
	linkHandler := identity
	if o.website {
		prepender = websitePrepender
		linkHandler = websiteLinkHandler
	}
	if _, err := os.Stat(o.path); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(o.path, os.ModeDir|os.ModePerm); err != nil {
			return err
		}
	}
	root.DisableAutoGenTag = !o.autogenTag
	return doc.GenMarkdownTreeCustom(root, o.path, prepender, linkHandler)
}
