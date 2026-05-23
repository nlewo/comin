package prometheus

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Prometheus struct {
	promRegistry             *prometheus.Registry
	buildInfo                *prometheus.GaugeVec
	deploymentInfo           *prometheus.GaugeVec
	fetchCounter             *prometheus.CounterVec
	isSuspended              prometheus.Gauge
	needToReboot             prometheus.Gauge
	lastFetchSuccessful      prometheus.Gauge
	lastBuildSuccessful      prometheus.Gauge
	lastDeploymentSuccessful prometheus.Gauge
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
	isSuspended := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "comin_is_suspended",
		Help: "Whether the host is suspended (1) or not (0).",
	})
	needToReboot := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "comin_need_to_reboot",
		Help: "Whether the host needs to reboot (1) or not (0).",
	})

	lastFetchSuccessful := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "last_fetch_successful",
		Help: "Whether the last fetch (any of the repositories) was successful (1) or not (0).",
	})
	lastBuildSuccessful := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "last_build_successful",
		Help: "Whether the last build was successful (1) or not (0).",
	})
	lastDeploymentSuccessful := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "last_deployment_successful",
		Help: "Whether the last deployment was successful (1) or not (0).",
	})
	promReg.MustRegister(buildInfo)
	promReg.MustRegister(deploymentInfo)
	promReg.MustRegister(fetchCounter)
	promReg.MustRegister(isSuspended)
	promReg.MustRegister(needToReboot)
	promReg.MustRegister(lastFetchSuccessful)
	promReg.MustRegister(lastBuildSuccessful)
	promReg.MustRegister(lastDeploymentSuccessful)
	return Prometheus{
		promRegistry:             promReg,
		buildInfo:                buildInfo,
		deploymentInfo:           deploymentInfo,
		fetchCounter:             fetchCounter,
		isSuspended:              isSuspended,
		needToReboot:             needToReboot,
		lastFetchSuccessful:      lastFetchSuccessful,
		lastBuildSuccessful:      lastBuildSuccessful,
		lastDeploymentSuccessful: lastDeploymentSuccessful,
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

func (m Prometheus) SetIsSuspended(isSuspended bool) {
	m.isSuspended.Set(boolToFloat64(isSuspended))
}

func (m Prometheus) SetNeedToReboot(needToReboot bool) {
	m.needToReboot.Set(boolToFloat64(needToReboot))
}

func (m Prometheus) SetLastFetchSuccessful(lastFetchSuccessful bool) {
	m.lastFetchSuccessful.Set(boolToFloat64(lastFetchSuccessful))
}
func (m Prometheus) SetLastBuildSuccessful(lastBuildSuccessful bool) {
	m.lastBuildSuccessful.Set(boolToFloat64(lastBuildSuccessful))
}
func (m Prometheus) SetLastDeploymentSuccessful(lastDeploymentSuccessful bool) {
	m.lastDeploymentSuccessful.Set(boolToFloat64(lastDeploymentSuccessful))
}
