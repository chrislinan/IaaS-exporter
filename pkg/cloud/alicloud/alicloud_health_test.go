package alicloud

import (
	"github.com/jarcoal/httpmock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
	"net/http"
	"testing"
)

var events = "{\n  \"data\": [\n    {\n      \"productId\": \"dummyProduct\",\n      \"title\": \"dummyTitile\",\n      \"currentStateSeverity\": \"ALARM\",\n      \"startTime\": 1665735021,\n      \"endTime\": 1665735058\n    },\n    {\n      \"productId\": \"dummyProduct2\",\n      \"title\": \"dummyTitile2\",\n      \"currentStateSeverity\": \"NOTIFICATION\",\n      \"startTime\": 1665734921,\n      \"endTime\": 1665735058\n    }\n  ],\n  \"total\": 0,\n  \"info\": \"成功处理\",\n  \"code\": 200,\n  \"success\": true,\n  \"httpCode\": 200,\n  \"requestId\": null\n}"

func TestAlicloudHealth(t *testing.T) {
	uri := "metrics"
	cred := vault.CloudCredentials{
		vault.AliCloudAccessKeyID:     "111",
		vault.AliCloudSecretAccessKey: "222",
	}
	vaultBackupBucket := config.VaultBackupBucketConfig{
		Bucket: "mock_bucket",
		Prefix: "mock_prefix",
	}
	conf := &config.Config{
		VaultBackupBucket: &vaultBackupBucket,
		Region:            "cn-shanghai",
	}

	httpmock.Activate()
	responderGetMetrics := httpmock.NewStringResponder(http.StatusOK, events)
	httpmock.RegisterResponder("GET", "https://status.aliyun.com/api/status/listProductEventForRegionInLast24Hours?regionId=cn-shanghai", responderGetMetrics)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		healthCollector := NewMetricsCollectorAliHealth(conf, cred, &log.Logger{})
		registry := prometheus.NewRegistry()
		registry.MustRegister(healthCollector)
		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	})
	assert.HTTPBodyContains(t, handler, "GET", uri, nil, "cpe_health_events{eventID=\"dummyTitile\",lastUpdatedTime=\"2022-10-14 08:10:58 +0000 UTC\",level=\"ALARM\",startTime=\"2022-10-14 08:10:21 +0000 UTC\"} 1")
	assert.HTTPBodyContains(t, handler, "GET", uri, nil, "cpe_health_events{eventID=\"dummyTitile2\",lastUpdatedTime=\"2022-10-14 08:10:58 +0000 UTC\",level=\"NOTIFICATION\",startTime=\"2022-10-14 08:08:41 +0000 UTC\"} 1")
}
