package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/nlewo/comin/internal/manager"
	"github.com/sirupsen/logrus"
)

func handlerStatus(m manager.Manager, w http.ResponseWriter, r *http.Request) {
	logrus.Infof("Getting status request %s from %s", r.URL, r.RemoteAddr)
	w.WriteHeader(http.StatusOK)
	s := m.GetState()
	logrus.Debugf("State is %#v", s)
	rJson, err := json.MarshalIndent(s, "", "\t")
	if err != nil {
		logrus.Error(err)
	}
	io.WriteString(w, string(rJson))
	return
}

func Serve(m manager.Manager, address string, port int) {
	handlerStatusFn := func(w http.ResponseWriter, r *http.Request) {
		handlerStatus(m, w, r)
		return
	}
	http.HandleFunc("/status", handlerStatusFn)
	url := fmt.Sprintf("%s:%d", address, port)
	logrus.Infof("Starting the webhook server on %s", url)
	if err := http.ListenAndServe(url, nil); err != nil {
		logrus.Errorf("Error while running the webhook server: %s", err)
		os.Exit(1)
	}
}
