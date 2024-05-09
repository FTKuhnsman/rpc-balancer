package rpcbalancer

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	logrus "github.com/sirupsen/logrus"
)

var (
	metrics    *Metrics
	nodeHealth bool
	log        *logrus.Logger
)

func init() {
	metrics = NewMetrics()
	nodeHealth = false
	log = logrus.New()
	log.Out = os.Stdout
	//log.SetFormatter(&logrus.TextFormatter{})

}

func SetLogLevel(level string) {
	switch level {
	case "info":
		log.SetLevel(logrus.InfoLevel)
	case "error":
		log.SetLevel(logrus.ErrorLevel)
	case "debug":
		log.SetLevel(logrus.DebugLevel)
		log.SetReportCaller(true)
	case "warn":
		log.SetLevel(logrus.WarnLevel)
	default:
		log.SetLevel(logrus.InfoLevel)
	}
}

func hexStringToInt64(hexStr string) (int64, error) {
	cleanedHexString := strings.TrimPrefix(hexStr, "0x")
	return strconv.ParseInt(cleanedHexString, 16, 64)
}

func HandleFavicon(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
