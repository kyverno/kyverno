package ttllabel

import "time"

func Validate(ttlValue string) error {
	_, err := time.ParseDuration(ttlValue)
	if err != nil {
		layoutRFCC := "2006-01-02T150405Z"
		// Try parsing ttlValue as a time in ISO 8601 format
		_, err := time.Parse(layoutRFCC, ttlValue)
		if err != nil {
			layoutCustom := "2006-01-02"
			_, err = time.Parse(layoutCustom, ttlValue)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
