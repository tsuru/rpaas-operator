package handlers

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
)

const timeout = time.Duration(1 * time.Second)

func HealthcheckHandler(w http.ResponseWriter, r *http.Request) {
	urls := r.URL.Query()["url"]

	if len(urls) < 1 {
		message := "missing parameter url"
		log.Println(message)
		http.Error(w, message, http.StatusBadRequest)
		return
	}

	for _, urlToCheck := range urls {
		urlToCheck, err := url.Parse(urlToCheck)

		if err != nil {
			message := fmt.Sprintf("url format error: %q", err)
			log.Printf(message)
			http.Error(w, message, http.StatusBadRequest)
			return
		}

		client := http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}
		resp, err := client.Get(urlToCheck.String())

		if err != nil {
			message := fmt.Sprintf("healthcheck request error: %q", err)
			log.Printf(message)
			http.Error(w, message, http.StatusServiceUnavailable)
			return
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 400 {
			message := fmt.Sprintf("unexpected status code: %d", resp.StatusCode)
			log.Printf(message)
			http.Error(w, message, http.StatusServiceUnavailable)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}
