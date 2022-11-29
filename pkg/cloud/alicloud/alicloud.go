package alicloud

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
	"net/http"
)

type AliExporter struct {
	quotaCollector  *MetricsCollectorAliQuota
	healthCollector *MetricsCollectorAliHealth
}

func (e *AliExporter) StartExporter(ctx context.Context, config *config.Config, credential vault.CloudCredentials, logger log.FieldLogger) {
	e.quotaCollector = NewMetricsCollectorAliQuota(config, credential, logger)
	prometheus.MustRegister(e.quotaCollector)
	e.healthCollector = NewMetricsCollectorAliHealth(config, credential, logger)
	prometheus.MustRegister(e.healthCollector)
	//http.HandleFunc(constant.VaultMonitorPath, func(w http.ResponseWriter, r *http.Request) {
	//	vaultBackupMonitorHandler(w, r, config, credential)
	//})
}

func (e *AliExporter) Scrape(ctx context.Context) {
	e.quotaCollector.scrape(ctx)
}

func vaultBackupMonitorHandler(w http.ResponseWriter, r *http.Request, config *config.Config, credential vault.CloudCredentials) {
	vaultBucketCollector := NewMetricsCollectorAliVaultBucket(config, credential)
	registry := prometheus.NewRegistry()
	registry.MustRegister(vaultBucketCollector)
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}
