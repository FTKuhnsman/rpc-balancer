package rpcbalancer

import (
	"log"
	"net/http"
	"strconv"
	"strings"
)

var (
	metrics    *Metrics
	nodeHealth bool
)

func init() {
	metrics = NewMetrics()
	nodeHealth = false
	log.SetFlags(log.Lshortfile)
}

func hexStringToInt64(hexStr string) (int64, error) {
	cleanedHexString := strings.TrimPrefix(hexStr, "0x")
	return strconv.ParseInt(cleanedHexString, 16, 64)
}

func HandleFavicon(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
