package command

import "strings"

func FormatDescription(short bool, url string, experimental bool, lines ...string) string {
	if len(lines) == 0 {
		return ""
	}
	description := lines[0]
	if short {
		return description
	}
	description += "\n"
	for _, line := range lines[1:] {
		description += "  "
		description += line
		description += "\n"
	}
	if experimental {
		description += "\n"
		description += "  "
		description += "NOTE: This is an experimental command, use `KYVERNO_EXPERIMENTAL=true` to enable it."
		description += "\n"
	}
	if url != "" {
		description += "\n"
		description += "  "
		description += "For more information visit " + url
		description += "\n"
	}
	return strings.TrimSpace(description)
}

func FormatExamples(in ...[]string) string {
	var examples string
	for _, example := range in {
		for _, line := range example {
			examples += "  "
			examples += line
			examples += "\n"
		}
		examples += "\n"
	}
	return strings.TrimRight(examples, " \n")
}
