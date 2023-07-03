package getter

import (
	"fmt"
	"os"

	gogettter "github.com/hashicorp/go-getter"
)

func Get(src string) (string, error) {
	dst, err := os.MkdirTemp("", "kubectl-kyverno")
	if err != nil {
		return "", err
	}
	fmt.Println(src, dst)
	return dst, gogettter.GetAny(dst, src, func(c *gogettter.Client) error {
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		c.Pwd = pwd
		return nil
	})
}
