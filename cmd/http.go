package cmd

import (
	"net/http"
	"time"

	"github.com/xiaoxiaolin0918/plumelog-cli/internal/client"
)

func clientHTTP(timeout time.Duration, jar *client.Jar) http.Client {
	return http.Client{Timeout: timeout, Jar: jar}
}
