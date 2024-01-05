package pluralize

func Pluralize(number int, singular string, plural string) string {
	if number == 1 {
		return singular
	}
	return plural
}
