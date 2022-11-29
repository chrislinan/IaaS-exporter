package gcp

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

var events = "[{\"id\":\"5Qmw8CdU6NxVRDFohwwT\",\"begin\":\"2022-10-07T13:13:48+00:00\",\"modified\":\"2022-10-07T21:30:03+00:00\",\"external_desc\":\"Connecting GitHub repository is not working\",\"most_recent_update\":{\"text\":\"The issue\",\"status\":\"UNAVAILABLE\"},\"status_impact\":\"SERVICE_INFORMATION\",\"severity\":\"low\",\"service_key\":\"zall\",\"service_name\":\"Multiple Products\",\"affected_products\":[{\"title\":\"Cloud Developer Tools\",\"id\":\"BGJQ6jbGK4kUuBTQFZ1G\"},{\"title\":\"Cloud Build\",\"id\":\"fw8GzBdZdqy4THau7e1y\"}],\"uri\":\"incidents/5Qmw8CdU6NxVRDFohwwT\",\"previously_affected_locations\":[{\"title\":\"Taiwan (asia-east1)\",\"id\":\"asia-east1\"},{\"title\":\"Hong Kong (asia-east2)\",\"id\":\"asia-east2\"},{\"title\":\"Tokyo (asia-northeast1)\",\"id\":\"asia-northeast1\"}]}," +
	"{\"id\":\"5Qmw8CdU6NxVRDFohwwT\",\"begin\":\"2022-10-07T13:13:48+00:00\",\"modified\":\"2022-10-07T21:30:03+00:00\",\"external_desc\":\"Connecting GitHub repository is not working\",\"most_recent_update\":{\"text\":\"The issue\",\"status\":\"AVAILABLE\"},\"status_impact\":\"SERVICE_INFORMATION\",\"severity\":\"low\",\"service_key\":\"zall\",\"service_name\":\"Multiple Products\",\"affected_products\":[{\"title\":\"Cloud Developer Tools\",\"id\":\"BGJQ6jbGK4kUuBTQFZ1G\"},{\"title\":\"Cloud Build\",\"id\":\"fw8GzBdZdqy4THau7e1y\"}],\"uri\":\"incidents/5Qmw8CdU6NxVRDFohwwT\",\"previously_affected_locations\":[{\"title\":\"Taiwan (asia-east1)\",\"id\":\"asia-east1\"},{\"title\":\"Hong Kong (asia-east2)\",\"id\":\"asia-east2\"}]}]"

func TestGcpHealth(t *testing.T) {
	uri := "/metrics"
	cred := vault.CloudCredentials{
		vault.GcpServiceAccount: "{\"type\": \"service_account\"}",
		vault.GcpProjectID:      "projectID",
	}
	vaultBackupBucket := config.VaultBackupBucketConfig{
		Bucket: "mock_bucket",
		Prefix: "mock_prefix",
	}
	conf := &config.Config{
		VaultBackupBucket: &vaultBackupBucket,
		Region:            "us-central1",
	}
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	responderGetMetrics := httpmock.NewStringResponder(http.StatusOK, events)
	httpmock.RegisterResponder("GET", "https://status.cloud.google.com/incidents.json", responderGetMetrics)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		healthCollector := NewMetricsCollectorGcpRmHealth(conf, cred, &log.Logger{})
		registry := prometheus.NewRegistry()
		registry.MustRegister(healthCollector)
		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	})

	assert.HTTPBodyContains(t, handler, "GET", uri, nil, "cpe_health_events{eventID=\"5Qmw8CdU6NxVRDFohwwT\",eventType=\"SERVICE_INFORMATION\",lastUpdatedTime=\"2022-10-07T21:30:03+00:00\",level=\"low\",serviceKey=\"zall\",serviceName=\"Multiple Products\",startTime=\"2022-10-07T13:13:48+00:00\",status=\"UNAVAILABLE\",title=\"Connecting GitHub repository is not working\",uri=\"incidents/5Qmw8CdU6NxVRDFohwwT\"} 1")
	assert.HTTPBodyContains(t, handler, "GET", uri, nil, "cpe_health_events_affected{affectedRegions=\"Taiwan (asia-east1),Hong Kong (asia-east2),Tokyo (asia-northeast1)\",affectedService=\"Cloud Developer Tools,Cloud Build\",eventID=\"5Qmw8CdU6NxVRDFohwwT\"} 1")
}
