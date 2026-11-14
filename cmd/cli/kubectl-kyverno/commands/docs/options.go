package docs

import (
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/spf13/pflag"
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
		if err := os.MkdirAll(o.path, os.ModeDir|os.ModePerm); err != nil {
			return err
		}
	}
	root.DisableAutoGenTag = !o.autogenTag

	if err := doc.GenMarkdownTreeCustom(root, o.path, prepender, linkHandler); err != nil {
		return err
	}

	return postProcessDocs(o.path, root)
}

func buildCmdMap(cmd *cobra.Command, cmdMap map[string]*cobra.Command) {
	filename := strings.ReplaceAll(cmd.CommandPath(), " ", "_") + ".md"
	cmdMap[filename] = cmd
	for _, child := range cmd.Commands() {
		if child.IsAvailableCommand() || child.Name() == "help" {
			buildCmdMap(child, cmdMap)
		}
	}
}

func postProcessDocs(dir string, root *cobra.Command) error {
	cmdMap := make(map[string]*cobra.Command)
	if root != nil {
		buildCmdMap(root, cmdMap)
	}

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		baseName := filepath.Base(path)
		if info.IsDir() || filepath.Ext(baseName) != ".md" {
			return nil
		}

		if baseName != "kyverno.md" && !strings.HasPrefix(baseName, "kyverno_") {
			return nil
		}

		targetCmd, ok := cmdMap[baseName]
		if !ok {
			return nil
		}

		return processCommandDoc(path, targetCmd)
	})
}

func processCommandDoc(path string, cmd *cobra.Command) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	hasOptions := false
	alreadyProcessed := false

	// PRE-SCAN FIX: Require exact "### Options" heading to prevent loose processing
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "### Options" {
			hasOptions = true
		}
		if hasOptions && strings.HasPrefix(trimmedLine, "* **<a id=") {
			alreadyProcessed = true
			break
		}
	}

	if !hasOptions {
		return nil
	}

	flagMap := make(map[string]*pflag.Flag)
	if cmd != nil {
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			flagMap[f.Name] = f
		})
		cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
			flagMap[f.Name] = f
		})
	}

	var newLines []string
	inOptions := false
	inInnerFence := false
	var activeFenceMarker string
	fencesRemoved := 0

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		if inOptions {
			currentMarker := ""
			if strings.HasPrefix(trimmedLine, "```") || strings.HasPrefix(trimmedLine, "~~~") {
				char := trimmedLine[0]
				count := 0
				for i := 0; i < len(trimmedLine); i++ {
					if trimmedLine[i] == char {
						count++
					} else {
						break
					}
				}
				currentMarker = trimmedLine[:count]
			}

			if currentMarker != "" {
				if !alreadyProcessed && fencesRemoved < 2 {
					fencesRemoved++
					continue
				}

				if !inInnerFence {
					inInnerFence = true
					activeFenceMarker = currentMarker
					newLines = append(newLines, line)
					continue
				}

				if inInnerFence && strings.HasPrefix(trimmedLine, activeFenceMarker) {
					isValidClose := true
					for i := 0; i < len(trimmedLine); i++ {
						if trimmedLine[i] != activeFenceMarker[0] {
							isValidClose = false
							break
						}
					}

					if isValidClose {
						inInnerFence = false
						activeFenceMarker = ""
						newLines = append(newLines, line)
						continue
					}
				}
			}

			if inInnerFence {
				newLines = append(newLines, line)
				continue
			}
		}

		if !inInnerFence && strings.HasPrefix(trimmedLine, "### ") {
			if len(newLines) > 0 && strings.TrimSpace(newLines[len(newLines)-1]) != "" {
				newLines = append(newLines, "")
			}

			if trimmedLine == "### Options" || trimmedLine == "### Options inherited from parent commands" {
				inOptions = true
				fencesRemoved = 0
				inInnerFence = false
				activeFenceMarker = ""
			} else {
				inOptions = false
			}

			newLines = append(newLines, line)
			continue
		}

		if inOptions {
			if trimmedLine == "" {
				newLines = append(newLines, line)
				continue
			}

			if strings.HasPrefix(trimmedLine, "* **<a id=") {
				newLines = append(newLines, line)
				continue
			}

			var matchedFlag *pflag.Flag
			if cmd != nil {
				parts := strings.Fields(trimmedLine)
				for _, part := range parts {
					if strings.HasPrefix(part, "--") {
						flagName := strings.TrimPrefix(part, "--")
						flagName = strings.Split(flagName, "=")[0]

						if f, exists := flagMap[flagName]; exists {
							matchedFlag = f
							break
						}
					}
				}
			}

			if matchedFlag != nil {
				anchor := matchedFlag.Name
				if anchor == "" {
					anchor = matchedFlag.Shorthand
				}

				flagToken := "--" + matchedFlag.Name
				idx := strings.Index(trimmedLine, flagToken)

				flagPart := ""
				descPart := ""

				if idx != -1 {
					sub := trimmedLine[idx:]
					parts := strings.SplitN(sub, "  ", 2)
					flagSyntaxEnd := idx + len(parts[0])
					flagPart = strings.TrimSpace(trimmedLine[:flagSyntaxEnd])
					if len(parts) > 1 {
						descPart = strings.TrimSpace(strings.Join(parts[1:], "  "))
					} else {
						flagPart = strings.TrimSpace(trimmedLine)
						descPart = matchedFlag.Usage
					}
				} else {
					flagPart = trimmedLine
					descPart = matchedFlag.Usage
				}

				escapedFlag := html.EscapeString(flagPart)
				escapedFlag = strings.ReplaceAll(escapedFlag, "*", `\*`)
				escapedFlag = strings.ReplaceAll(escapedFlag, "_", `\_`)

				newLine := fmt.Sprintf("* **<a id=\"%s\"></a>%s**: %s", anchor, escapedFlag, descPart)
				newLines = append(newLines, newLine)
			} else {
				newLines = append(newLines, line)
			}
		} else {
			newLines = append(newLines, line)
		}
	}

	resolvedContent := strings.Join(newLines, "\n")
	return os.WriteFile(path, []byte(resolvedContent), 0644)
}
