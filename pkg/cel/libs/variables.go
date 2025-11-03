package libs

type Exception struct {
	AllowedImages []string `cel:"allowedImages"`
	AllowedValues []string `cel:"allowedValues"`
}
