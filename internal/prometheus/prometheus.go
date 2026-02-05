package prometheus

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Prometheus struct {
	promRegistry   *prometheus.Registry
	buildInfo      *prometheus.GaugeVec
	deploymentInfo *prometheus.GaugeVec
	fetchCounter   *prometheus.CounterVec
	hostInfo       *prometheus.GaugeVec
}

func New() Prometheus {
	promReg := prometheus.NewRegistry()
	buildInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "comin_build_info",
		Help: "Build info for comin.",
	}, []string{"version"})
	deploymentInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "comin_deployment_info",
		Help: "Info of the last deployment.",
	}, []string{"commit_id", "status"})
	fetchCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "comin_fetch_count",
		Help: "Number of fetches per status",
	}, []string{"remote_name", "status"})
	hostInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "comin_host_info",
		Help: "Info of the host.",
	}, []string{"is_suspended", "need_to_reboot"})
	promReg.MustRegister(buildInfo)
	promReg.MustRegister(deploymentInfo)
	promReg.MustRegister(fetchCounter)
	promReg.MustRegister(hostInfo)
	return Prometheus{
		promRegistry:   promReg,
		buildInfo:      buildInfo,
		deploymentInfo: deploymentInfo,
		fetchCounter:   fetchCounter,
		hostInfo:       hostInfo,
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

func (m Prometheus) SetBuildInfo(version string) {
	m.buildInfo.Reset()
	m.buildInfo.With(prometheus.Labels{"version": version}).Set(1)
}

func (m Prometheus) SetDeploymentInfo(commitId, status string) {
	m.deploymentInfo.Reset()
	m.deploymentInfo.With(prometheus.Labels{"commit_id": commitId, "status": status}).Set(1)
}

func boolToString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func (m Prometheus) SetHostInfo(needToReboot bool, isSuspended bool) {
	m.hostInfo.Reset()
	m.hostInfo.With(prometheus.Labels{
		"need_to_reboot": boolToString(needToReboot),
		"is_suspended":   boolToString(isSuspended),
	}).Set(1)
}
