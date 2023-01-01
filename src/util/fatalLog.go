package util

import (
	log "github.com/sirupsen/logrus"
	"lombok-plugin-action/src/config"
	"os"
)

func FatalLogf(format string, args ...interface{}) {
	log.Errorf(format, args...)
	checkExit()
}

func FatalLogln(args ...interface{}) {
	log.Errorln(args...)
	checkExit()
}

func checkExit() {
	if !config.KeepWhenException {
		os.Exit(1)
	}
}
