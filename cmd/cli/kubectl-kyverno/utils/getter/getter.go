package getter

import (
	"os"

	gogettter "github.com/hashicorp/go-getter"
)

func Get(src string) (string, error) {
	if _, err := os.Stat(src); err == nil {
		return src, nil
	}
	dst, err := os.MkdirTemp("", "kubectl-kyverno")
	if err != nil {
		return "", err
	}
	return dst, gogettter.GetAny(dst, src, func(c *gogettter.Client) error {
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		c.Pwd = pwd
		return nil
	})
}
