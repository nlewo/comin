package prometheus

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Prometheus struct {
	promRegistry   *prometheus.Registry
	deploymentInfo *prometheus.GaugeVec
	fetchCounter   *prometheus.CounterVec
}

func New() Prometheus {
	promReg := prometheus.NewRegistry()
	deploymentInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "comin_deployment_info",
		Help: "Info of the last deployment.",
	}, []string{"commit_id", "status"})
	fetchCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "comin_fetch_count",
		Help: "Number of fetches per status",
	}, []string{"remote_name", "status"})
	promReg.MustRegister(deploymentInfo)
	promReg.MustRegister(fetchCounter)
	return Prometheus{
		promRegistry:   promReg,
		deploymentInfo: deploymentInfo,
		fetchCounter:   fetchCounter,
	}
}

func (m Prometheus) Handler() http.Handler {
	return promhttp.HandlerFor(
		m.promRegistry,
		promhttp.HandlerOpts{
			EnableOpenMetrics: false,
		})
}

func (m Prometheus) IncFetchCounter(remoteName, status string) {
	m.fetchCounter.With(prometheus.Labels{"remote_name": remoteName, "status": status}).Inc()
}

func (m Prometheus) SetDeploymentInfo(commitId, status string) {
	m.deploymentInfo.Reset()
	m.deploymentInfo.With(prometheus.Labels{"commit_id": commitId, "status": status}).Set(1)
}
