package http

import (
	"fmt"
	"net/http"
	"os"

	"github.com/nlewo/comin/internal/manager"
	"github.com/nlewo/comin/internal/prometheus"
	"github.com/sirupsen/logrus"
)

// Serve starts http servers. We create two HTTP servers to easily be
// able to expose metrics publicly while keeping on localhost only the
// API.
func Serve(m *manager.Manager, p prometheus.Prometheus, apiAddress string, apiPort int, metricsAddress string, metricsPort int) {
	muxMetrics := http.NewServeMux()
	muxMetrics.Handle("/metrics", p.Handler())
	go func() {
		url := fmt.Sprintf("%s:%d", metricsAddress, metricsPort)
		logrus.Infof("http: starting the metrics server on %s", url)
		if err := http.ListenAndServe(url, muxMetrics); err != nil {
			logrus.Errorf("Error while running the metrics server: %s", err)
			os.Exit(1)
		}
	}()
}
