package alicloud

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
	"time"
)

type MetricsCollectorAliHealth struct {
	conf *config.Config
	cred vault.CloudCredentials
}

func NewMetricsCollectorAliHealth(config *config.Config, cred vault.CloudCredentials, logger log.FieldLogger) *MetricsCollectorAliHealth {

	m := &MetricsCollectorAliHealth{}
	m.conf = config
	m.cred = cred

	eventLabel := []string{
		"eventID",
		"startTime",
		"lastUpdatedTime",
		"level",
	}

	entityLabel := []string{
		"eventID",
		"affectedService",
		"affectedRegions",
	}
	eventOpenTotalLabel := []string{"eventType"}
	eventCloseTotalLabel := []string{"eventType"}

	common.InitHealthCounterVec("AliCloud", eventLabel, entityLabel, eventOpenTotalLabel, eventCloseTotalLabel)
	return m
}

func (m *MetricsCollectorAliHealth) Describe(ch chan<- *prometheus.Desc) {
	common.HealthEvent.Describe(ch)
	common.AffectedEntity.Describe(ch)
}

func (m *MetricsCollectorAliHealth) Collect(ch chan<- prometheus.Metric) {
	eventUrl := "https://status.aliyun.com/api/status/listProductEventForRegionInLast24Hours?regionId="
	region := m.conf.Region

	eventUrl += region
	eventResp, err := http.Get(eventUrl)

	if err != nil {
		log.Fatal("An error occurred sending http request:", err)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Errorf("An error occurred: %v", err)
		}
	}(eventResp.Body)

	eventRespBody, err := ioutil.ReadAll(eventResp.Body)

	if err != nil {
		log.Fatal("An error occurred reading response body", err)
	}

	eventResults := make(map[string]interface{})
	err = json.Unmarshal(eventRespBody, &eventResults)
	if err != nil {
		log.Fatal("An error occurred during unmarshal response body", err)
	}

	if eventResults["success"] == true {
		for _, result := range eventResults["data"].([]interface{}) {
			productId := result.(map[string]interface{})["productId"]
			title := result.(map[string]interface{})["title"]
			currentStateSeverity := result.(map[string]interface{})["currentStateSeverity"]
			startTime := time.Unix(int64(result.(map[string]interface{})["startTime"].(float64)), 0).UTC()
			endTime := time.Unix(int64(result.(map[string]interface{})["endTime"].(float64)), 0).UTC()
			common.HealthEvent.WithLabelValues(title.(string), startTime.String(), endTime.String(), currentStateSeverity.(string)).Inc()
			common.AffectedEntity.WithLabelValues(title.(string), productId.(string), region).Inc()
		}
	}

	common.HealthEvent.Collect(ch)
	common.AffectedEntity.Collect(ch)
}
