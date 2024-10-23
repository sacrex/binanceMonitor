package log

import (
	"github.com/crazygit/binance-market-monitor/helper"
	"github.com/sirupsen/logrus"
	"os"
	"strings"
)

var log = logrus.New()

func init() {
	bytes, err := os.ReadFile("./.env")
	if err != nil {
		log.Panic("read env error")
		return
	}

	envArr := strings.Split(string(bytes), "\r\n")
	for _, item := range envArr {
		kv := strings.Split(item, "=")
		if len(kv) == 2 {
			err = os.Setenv(kv[0], kv[1])
		}
	}

	if helper.IsProductionEnvironment() {
		log.SetFormatter(&logrus.JSONFormatter{})
		log.SetLevel(logrus.InfoLevel)
	} else {
		log.SetLevel(logrus.DebugLevel)
	}
	log.SetOutput(os.Stdout)
}

func GetLog() *logrus.Logger {
	return log
}
