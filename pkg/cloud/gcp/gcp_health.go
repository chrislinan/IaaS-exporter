package gcp

import (
	"encoding/json"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/common"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

type MetricsCollectorGcpRmHealth struct {
	conf *config.Config
	cred vault.CloudCredentials
}

func NewMetricsCollectorGcpRmHealth(config *config.Config, cred vault.CloudCredentials, logger log.FieldLogger) *MetricsCollectorGcpRmHealth {
	m := &MetricsCollectorGcpRmHealth{}
	m.conf = config
	m.cred = cred

	eventLabel := []string{
		"eventID",
		"title",
		"startTime",
		"lastUpdatedTime",
		"status",
		"eventType",
		"level",
		"uri",
		"serviceKey",
		"serviceName",
	}

	entityLabel := []string{
		"eventID",
		"affectedService",
		"affectedRegions",
	}

	eventOpenTotalLabel := []string{"eventType"}
	eventCloseTotalLabel := []string{"eventType"}
	common.InitHealthCounterVec("Gcp", eventLabel, entityLabel, eventOpenTotalLabel, eventCloseTotalLabel)
	return m
}

func (m *MetricsCollectorGcpRmHealth) Describe(ch chan<- *prometheus.Desc) {
	common.HealthEvent.Describe(ch)
	common.AffectedEntity.Describe(ch)
	common.HealthOpenedTotal.Describe(ch)
	common.HealthClosedTotal.Describe(ch)
}

func (m *MetricsCollectorGcpRmHealth) Collect(ch chan<- prometheus.Metric) {
	common.HealthOpenedTotal.Reset()
	common.HealthClosedTotal.Reset()
	common.HealthEvent.Reset()
	common.AffectedEntity.Reset()
	url := "https://status.cloud.google.com/incidents.json"
	resp, err := http.Get(url)
	if err != nil {
		log.Fatal("An error occurred during send http request:", err)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Errorf("An error occured: %v", err)
		}
	}(resp.Body)

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("An error occurred during read response body", err)
	}

	var results []map[string]interface{}
	err = json.Unmarshal(respBody, &results)
	if err != nil {
		log.Fatal("An error occurred during unmarshal response body", err)
	}

	for _, result := range results {
		status := result["most_recent_update"].(map[string]interface{})["status"].(string)
		eventID := result["id"].(string)
		title := result["external_desc"].(string)
		startTime := result["begin"].(string)
		lastUpdateTime := result["modified"].(string)
		statusImpact := result["status_impact"].(string)
		level := result["severity"].(string)
		uri := result["uri"].(string)
		serviceKey := result["service_key"].(string)
		serviceName := result["service_name"].(string)
		common.HealthEvent.WithLabelValues(eventID, title, startTime, lastUpdateTime, status, statusImpact,
			level, uri, serviceKey, serviceName).Inc()
		if status == "AVAILABLE" {
			common.HealthClosedTotal.WithLabelValues(statusImpact).Inc()
		} else {
			common.HealthOpenedTotal.WithLabelValues(statusImpact).Inc()
		}
		products := result["affected_products"].([]interface{})
		affectedProducts := make([]string, 0)
		for _, product := range products {
			affectedProducts = append(affectedProducts, product.(map[string]interface{})["title"].(string))
		}
		locations := result["previously_affected_locations"].([]interface{})
		affectedLocations := make([]string, 0)
		for _, location := range locations {
			affectedLocations = append(affectedLocations, location.(map[string]interface{})["title"].(string))
		}
		common.AffectedEntity.WithLabelValues(eventID, strings.Join(affectedProducts, ","), strings.Join(affectedLocations, ",")).Inc()
	}
	common.HealthEvent.Collect(ch)
	common.AffectedEntity.Collect(ch)
	common.HealthOpenedTotal.Collect(ch)
	common.HealthClosedTotal.Collect(ch)
}
