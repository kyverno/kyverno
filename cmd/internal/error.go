package internal

import (
	"fmt"
	"os"

	"github.com/go-logr/logr"
)

func checkErr(err error, msg string) {
	if err != nil {
		fmt.Println(msg, err)
		os.Exit(1)
	}
}

func checkError(logger logr.Logger, err error, msg string, keysAndValues ...interface{}) {
	if err != nil {
		logger.Error(err, msg, keysAndValues...)
		os.Exit(1)
	}
}
