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
	// TODO: deprecated: remove for the next release
	hostInfo     *prometheus.GaugeVec
	isSuspended  prometheus.Gauge
	needToReboot prometheus.Gauge
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
	// TODO: deprecated: remove for the next release
	hostInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "comin_host_info",
		Help: "(DEPRECATED) Info of the host.",
	}, []string{"is_suspended", "need_to_reboot"})
	isSuspended := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "comin_is_suspended",
		Help: "Whether the host is suspended (1) or not (0).",
	})
	needToReboot := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "comin_need_to_reboot",
		Help: "Whether the host needs to reboot (1) or not (0).",
	})
	promReg.MustRegister(buildInfo)
	promReg.MustRegister(deploymentInfo)
	promReg.MustRegister(fetchCounter)
	// TODO: deprecated: remove for the next release
	promReg.MustRegister(hostInfo)
	promReg.MustRegister(isSuspended)
	promReg.MustRegister(needToReboot)
	return Prometheus{
		promRegistry:   promReg,
		buildInfo:      buildInfo,
		deploymentInfo: deploymentInfo,
		fetchCounter:   fetchCounter,
		// TODO: deprecated: remove for the next release
		hostInfo:     hostInfo,
		isSuspended:  isSuspended,
		needToReboot: needToReboot,
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

func boolToFloat64(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// TODO: deprecated: remove for the next release
func (m Prometheus) SetHostInfo(needToReboot bool, isSuspended bool) {
	m.hostInfo.Reset()
	m.hostInfo.With(prometheus.Labels{
		"need_to_reboot": boolToString(needToReboot),
		"is_suspended":   boolToString(isSuspended),
	}).Set(1)
}

func (m Prometheus) SetIsSuspended(isSuspended bool) {
	m.isSuspended.Set(boolToFloat64(isSuspended))
}

func (m Prometheus) SetNeedToReboot(needToReboot bool) {
	m.needToReboot.Set(boolToFloat64(needToReboot))
}
