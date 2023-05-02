package http

import (
	"fmt"
	"github.com/nlewo/comin/types"
	"github.com/nlewo/comin/worker"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"os"
)

func handlerStatus(stateFilepath string, w http.ResponseWriter, r *http.Request) {
	logrus.Infof("Getting status request %s from %s", r.URL, r.RemoteAddr)
	w.WriteHeader(http.StatusOK)
	content, err := ioutil.ReadFile(stateFilepath)
	if err != nil {
		logrus.Debugf("Error while reading the statefile: ", err)
		return
	}
	io.WriteString(w, string(content))
	return
}

func Run(worker worker.Worker, cfg types.Webhook, stateFilepath string) {
	handlerStatusFn := func(w http.ResponseWriter, r *http.Request) {
		handlerStatus(stateFilepath, w, r)
		return
	}
	handler := func(w http.ResponseWriter, r *http.Request) {
		var secret string
		logrus.Infof("Getting webhook request %s from %s", r.URL, r.RemoteAddr)
		if cfg.Secret != "" {
			secret = r.Header.Get("X-Gitlab-Token")
			if secret == "" {
				logrus.Infof("Webhook called from %s without the X-Gitlab-Token header", r.RemoteAddr)
				w.WriteHeader(http.StatusUnauthorized)
				io.WriteString(w, "The header X-Gitlab-Token is required\n")
				return
			}
			if secret != cfg.Secret {
				logrus.Infof("Webhook called from %s with the invalid secret %s", r.RemoteAddr, secret)
				w.WriteHeader(http.StatusUnauthorized)
				io.WriteString(w, "Invalid X-Gitlab-Token header value\n")
				return
			}
		}
		if worker.Beat() {
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "A deployment has been triggered\n")
		} else {
			w.WriteHeader(http.StatusConflict)
			io.WriteString(w, "A deployment is already running\n")
		}
	}
	http.HandleFunc("/deploy", handler)
	http.HandleFunc("/status", handlerStatusFn)
	url := fmt.Sprintf("%s:%d", cfg.Address, cfg.Port)
	logrus.Infof("Starting the webhook server on %s", url)
	if err := http.ListenAndServe(url, nil); err != nil {
		logrus.Errorf("Error while running the webhook server: %s", err)
		os.Exit(1)
	}
}
