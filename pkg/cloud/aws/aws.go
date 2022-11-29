package aws

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/aws/health"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/aws/monitor"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/aws/quota"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/aws/vault_bucket"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
	"net/http"
)

type AwsExporter struct {
	quotaCollector      *quota.MetricsCollectorAwsQuota
	healthCollector     *health.MetricsCollectorAwsHealth
	cloudWatchCollector *monitor.MetricsCollectorAwsMonitor
	bucketCollector     *vault_bucket.MetricsCollectorAWSVaultBucket
}

func (e *AwsExporter) StartExporter(ctx context.Context, config *config.Config, credential vault.CloudCredentials, logger log.FieldLogger) {
	e.quotaCollector = quota.NewMetricsCollectorAwsQuota(config, credential, logger)
	prometheus.MustRegister(e.quotaCollector)

	//e.healthCollector = health.NewMetricsCollectorAwsHealth(config, credential, logger)
	//prometheus.MustRegister(e.healthCollector)
	//
	//e.cloudWatchCollector = monitor.NewMetricsCollectorAwsMonitor(config, credential, logger)
	//http.HandleFunc(constant.MetricsMonitorPath, func(w http.ResponseWriter, r *http.Request) {
	//	handler(w, r, e.cloudWatchCollector)
	//})

	//if config.Project == "hc-vault" {
	//	e.bucketCollector = vault_bucket.NewMetricsCollectorAWSVaultBucket(config, credential, logger)
	//	http.HandleFunc(constant.VaultMonitorPath, func(w http.ResponseWriter, r *http.Request) {
	//		handler(w, r, e.bucketCollector)
	//	})
	//}
}

func (e *AwsExporter) Scrape(ctx context.Context) {
	e.quotaCollector.Scrape(ctx)
	//e.cloudWatchCollector.Scrape(ctx)
}

func handler(w http.ResponseWriter, r *http.Request, collector prometheus.Collector) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}
