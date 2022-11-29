package gcp

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
	"net/http"
)

type GcpExporter struct {
	quotaCollector  *MetricsCollectorGcpRmQuota
	healthCollector *MetricsCollectorGcpRmHealth
}

func (e *GcpExporter) StartExporter(ctx context.Context, config *config.Config, credential vault.CloudCredentials, logger log.FieldLogger) {
	e.quotaCollector = NewMetricsCollectorGcpRmQuota(config, credential, logger)
	prometheus.MustRegister(e.quotaCollector)
	//e.healthCollector = NewMetricsCollectorGcpRmHealth(config, credential, logger)
	//prometheus.MustRegister(e.healthCollector)
	//http.HandleFunc(constant.VaultMonitorPath, func(w http.ResponseWriter, r *http.Request) {
	//	vaultBackupMonitorHandler(w, r, config, cred)
	//})
}

func (e *GcpExporter) Scrape(ctx context.Context) {
	e.quotaCollector.scrape(ctx)
}

func vaultBackupMonitorHandler(w http.ResponseWriter, r *http.Request, config *config.Config, credential vault.CloudCredentials) {
	vaultBucketCollector := NewMetricsCollectorGcpVaultBucket(config, credential)
	registry := prometheus.NewRegistry()
	registry.MustRegister(vaultBucketCollector)
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}
