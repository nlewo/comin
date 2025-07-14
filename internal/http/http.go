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

func handlerStatus(m *manager.Manager, w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("Getting status request %s from %s", r.URL, r.RemoteAddr)
	w.WriteHeader(http.StatusOK)
	s := m.GetState()
	logrus.Debugf("Manager state is %#v", s)
	rJson, err := json.MarshalIndent(s, "", "\t")
	if err != nil {
		logrus.Error(err)
	}
	_, _ = io.Writer.Write(w, rJson)
}

// Serve starts http servers. We create two HTTP servers to easily be
// able to expose metrics publicly while keeping on localhost only the
// API.
func Serve(m *manager.Manager, p prometheus.Prometheus, apiAddress string, apiPort int, metricsAddress string, metricsPort int) {
	handlerStatusFn := func(w http.ResponseWriter, r *http.Request) {
		handlerStatus(m, w, r)
	}
	handlerFetcherFn := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		s := m.GetState().Fetcher
		rJson, _ := json.MarshalIndent(s, "", "\t")
		_, _ = io.Writer.Write(w, rJson)
	}
	handlerFetcherFetchFn := func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			s := m.GetState().Fetcher
			remotes := make([]string, 0)
			for _, r := range s.RepositoryStatus.Remotes {
				remotes = append(remotes, r.Name)
			}
			m.Fetcher.TriggerFetch(remotes)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
	handlerBuilderSuspendFn := func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			if err := m.Builder.Suspend(); err != nil {
				w.WriteHeader(http.StatusConflict)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
	handlerBuilderResumeFn := func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			if err := m.Builder.Resume(); err != nil {
				w.WriteHeader(http.StatusConflict)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
	handlerManagerSuspendFn := func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			if err := m.Suspend(); err != nil {
				w.WriteHeader(http.StatusConflict)
				_, _ = io.Writer.Write(w, []byte(err.Error()))
			} else {
				w.WriteHeader(http.StatusOK)
			}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
	handlerManagerResumeFn := func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			if err := m.Resume(); err != nil {
				w.WriteHeader(http.StatusConflict)
				_, _ = io.Writer.Write(w, []byte(err.Error()))
			} else {
				w.WriteHeader(http.StatusOK)
			}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}

	muxApi := http.NewServeMux()
	muxApi.HandleFunc("/api/status", handlerStatusFn)
	muxApi.HandleFunc("/api/fetcher", handlerFetcherFn)
	muxApi.HandleFunc("/api/fetcher/fetch", handlerFetcherFetchFn)
	muxApi.HandleFunc("/api/builder/suspend", handlerBuilderSuspendFn)
	muxApi.HandleFunc("/api/builder/resume", handlerBuilderResumeFn)
	muxApi.HandleFunc("/api/manager/suspend", handlerManagerSuspendFn)
	muxApi.HandleFunc("/api/manager/resume", handlerManagerResumeFn)

	muxMetrics := http.NewServeMux()
	muxMetrics.Handle("/metrics", p.Handler())

	go func() {
		url := fmt.Sprintf("%s:%d", apiAddress, apiPort)
		logrus.Infof("Starting the API server on %s", url)
		if err := http.ListenAndServe(url, muxApi); err != nil {
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
