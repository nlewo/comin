package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/nlewo/comin/internal/manager"
	"github.com/nlewo/comin/internal/prometheus"
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

func handlerDeploymentList(m manager.Manager, w http.ResponseWriter, r *http.Request) {
	logrus.Infof("Getting deployment from %s", r.RemoteAddr)
	w.WriteHeader(http.StatusOK)
	d := m.GetDeploymentList()
	logrus.Debugf("Deployment list is %#v", d)
	rJson, err := json.MarshalIndent(d, "", "\t")
	if err != nil {
		logrus.Error(err)
	}
	io.WriteString(w, string(rJson))
	return
}

// Serve starts http servers. We create two HTTP servers to easily be
// able to expose metrics publicly while keeping on localhost only the
// API.
func Serve(m manager.Manager, p prometheus.Prometheus, apiAddress string, apiPort int, metricsAddress string, metricsPort int) {
	handlerStatusFn := func(w http.ResponseWriter, r *http.Request) {
		handlerStatus(m, w, r)
		return
	}
	handlerDeploymentListFn := func(w http.ResponseWriter, r *http.Request) {
		handlerDeploymentList(m, w, r)
		return
	}

	muxStatus := http.NewServeMux()
	muxStatus.HandleFunc("/status", handlerStatusFn)
	muxStatus.HandleFunc("/deployment", handlerDeploymentListFn)
	muxMetrics := http.NewServeMux()
	muxMetrics.Handle("/metrics", p.Handler())

	go func() {
		url := fmt.Sprintf("%s:%d", apiAddress, apiPort)
		logrus.Infof("Starting the API server on %s", url)
		if err := http.ListenAndServe(url, muxStatus); err != nil {
			logrus.Errorf("Error while running the API server: %s", err)
			os.Exit(1)
		}
	}()
	go func() {
		url := fmt.Sprintf("%s:%d", metricsAddress, metricsPort)
		logrus.Infof("Starting the metrics server on %s", url)
		if err := http.ListenAndServe(url, muxMetrics); err != nil {
			logrus.Errorf("Error while running the metrics server: %s", err)
			os.Exit(1)
		}
	}()
}
