package docs

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

type options struct {
	path       string
	website    bool
	autogenTag bool
	noDate     bool
	mdLinks    bool
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
		prepender = websitePrepender(o.noDate)
		if !o.mdLinks {
			linkHandler = websiteLinkHandler
		}
	}
	if _, err := os.Stat(o.path); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(o.path, 0o750); err != nil {
			return err
		}
	}
	root.DisableAutoGenTag = !o.autogenTag
	return genMarkdownTree(root, o.path, prepender, linkHandler)
}

// genMarkdownTree is doc.GenMarkdownTreeCustom, except that each file is
// created with 0o600 instead of the 0o666 used by os.Create, so generated docs
// are not writable by other users when the umask is permissive.
func genMarkdownTree(cmd *cobra.Command, dir string, filePrepender, linkHandler func(string) string) error {
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}
		if err := genMarkdownTree(c, dir, filePrepender, linkHandler); err != nil {
			return err
		}
	}
	filename := filepath.Join(dir, strings.ReplaceAll(cmd.CommandPath(), " ", "_")+".md")
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) // #nosec G304
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.WriteString(f, filePrepender(filename)); err != nil {
		return err
	}
	return doc.GenMarkdownCustom(cmd, f, linkHandler)
}
