package azure

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

var events = "{\n    \"value\": [\n      {\n        \"id\": \"dummyID\",\n        \"name\": \"JVJC-V88\",\n        \"type\": \"Microsoft.ResourceHealth/events\",\n        \"properties\": {\n          \"eventType\": \"ServiceIssue\",\n          \"eventSource\": \"ServiceHealth\",\n          \"status\": \"Resolved\",\n          \"title\": \"dummyTitle\",\n          \"summary\": \"dummySummary\",\n          \"description\": \"description\",\n          \"platformInitiated\": true,\n          \"header\": \"Your service might have been impacted by an Azure service issue\",\n          \"level\": \"Warning\",\n          \"eventLevel\": \"Informational\",\n          \"impactStartTime\": \"2022-09-23T07:17:01.197Z\",\n          \"impactMitigationTime\": \"2022-09-23T11:00:37Z\",\n          \"impact\": [\n            {\n              \"impactedService\": \"Azure Active Directory\",\n              \"impactedRegions\": [\n                {\n                  \"impactedRegion\": \"Global\",\n                  \"status\": \"Resolved\",\n                  \"impactedSubscriptions\": [\n                    \"a68ae472-1849-4ed9-a700-24f5070acd2d\"\n                  ],\n                  \"impactedTenants\": [],\n                  \"lastUpdateTime\": \"2022-10-06T23:34:40.7648539Z\",\n                  \"updates\": [\n                    {\n                      \"summary\": \"summary\",\n                      \"updateDateTime\": \"2022-10-06T23:34:40.7648539Z\"\n                    }\n                  ]\n                }\n              ]\n            }\n          ],\n          \"isHIR\": false,\n          \"priority\": 19,\n          \"lastUpdateTime\": \"2022-10-06T23:34:40.7648539Z\"\n        }\n      }\n    ]\n  }"

func TestAzureHealth(t *testing.T) {
	uri := "/metrics"
	cred := vault.CloudCredentials{
		vault.AzureClientSecret:   "clientSecret",
		vault.AzureClientID:       "clientID",
		vault.AzureTenantID:       "tenantID",
		vault.AzureSubscriptionID: "subscriptionID",
	}

	conf := &config.Config{
		Region: "dummy_region",
		Azure: &config.AzureConfig{
			SubscriptionID: "a68ae472-1849-4ed9-a700-24f5070acd2d",
		},
	}
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	responderGetToken := httpmock.NewStringResponder(http.StatusOK, `{"access_token": "dummyToken"}`)
	httpmock.RegisterResponder("POST", "https://login.microsoftonline.com/tenantID/oauth2/v2.0/token", responderGetToken)

	responderGetMetrics := httpmock.NewStringResponder(http.StatusOK, events)
	httpmock.RegisterResponder("GET", "https://management.azure.com/subscriptions/a68ae472-1849-4ed9-a700-24f5070acd2d/providers/Microsoft.ResourceHealth/events?api-version=2018-07-01", responderGetMetrics)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		healthCollector := NewMetricsCollectorAzureRmHealth(conf, cred, &log.Logger{})
		registry := prometheus.NewRegistry()
		registry.MustRegister(healthCollector)
		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	})

	assert.HTTPBodyContains(t, handler, "GET", uri, nil, "cpe_health_events{eventID=\"dummyID\",eventType=\"ServiceIssue\",lastUpdatedTime=\"2022-10-06T23:34:40.7648539Z\",level=\"Warning\",startTime=\"2022-09-23T07:17:01.197Z\",status=\"Resolved\",title=\"dummyTitle\"} 1")
	assert.HTTPBodyContains(t, handler, "GET", uri, nil, "cpe_health_events_affected{affectedRegions=\"Global\",affectedService=\"Azure Active Directory\",eventID=\"dummyID\"} 1")
}
