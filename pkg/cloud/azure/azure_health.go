package azure

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
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

type MetricsCollectorAzureRmHealth struct {
	authorizer autorest.Authorizer
	conf       *config.Config
	cred       vault.CloudCredentials
}

func NewMetricsCollectorAzureRmHealth(config *config.Config, cred vault.CloudCredentials, logger log.FieldLogger) *MetricsCollectorAzureRmHealth {
	clientCredentialConfig := auth.NewClientCredentialsConfig(cred[vault.AzureClientID], cred[vault.AzureClientSecret], cred[vault.AzureTenantID])
	authorizer, err := clientCredentialConfig.Authorizer()
	if err != nil {
		log.Print("Error while getting authorizer: ", err)
	}
	m := &MetricsCollectorAzureRmHealth{}
	m.conf = config
	m.authorizer = authorizer
	m.cred = cred

	eventLabel := []string{
		"eventID",
		"title",
		"eventType",
		"lastUpdatedTime",
		"startTime",
		"status",
		"level",
	}

	entityLabel := []string{
		"eventID",
		"affectedService",
		"affectedRegions",
	}
	eventOpenTotalLabel := []string{"eventType"}
	eventCloseTotalLabel := []string{"eventType"}
	common.InitHealthCounterVec("Azure", eventLabel, entityLabel, eventOpenTotalLabel, eventCloseTotalLabel)
	return m
}

func (m *MetricsCollectorAzureRmHealth) Describe(ch chan<- *prometheus.Desc) {
	common.HealthEvent.Describe(ch)
	common.AffectedEntity.Describe(ch)
	common.HealthOpenedTotal.Describe(ch)
	common.HealthClosedTotal.Describe(ch)
}

func (m *MetricsCollectorAzureRmHealth) Collect(ch chan<- prometheus.Metric) {
	common.HealthOpenedTotal.Reset()
	common.HealthClosedTotal.Reset()
	common.HealthEvent.Reset()
	common.AffectedEntity.Reset()
	url := fmt.Sprintf("https://management.azure.com/subscriptions/%s/providers/Microsoft.ResourceHealth/events?api-version=2018-07-01", m.conf.Azure.SubscriptionID)
	client := &http.Client{}

	token, err := getToken(m.cred[vault.AzureTenantID], m.cred[vault.AzureClientID], m.cred[vault.AzureClientSecret])
	if token == "" || err != nil {
		log.Fatal("An error occurred during get token:", err)
	}

	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatal("An Error occurred during create http request:", err)
	}
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	request.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(request)
	if err != nil {
		log.Fatal("An error occurred during send http request:", err)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Errorf("An error occured: %v", err)
		}
	}(resp.Body)
	respbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("An error occurred during read response body", err)
	}

	result := make(map[string]interface{})
	err = json.Unmarshal(respbody, &result)
	if err != nil {
		log.Fatal("An error occurred during unmarshal response body", err)
	}
	for _, val := range result["value"].([]interface{}) {
		eventID := val.(map[string]interface{})["id"]
		property := val.(map[string]interface{})["properties"]
		status := property.(map[string]interface{})["status"]
		lastUpdateTime := property.(map[string]interface{})["lastUpdateTime"]
		eventType := property.(map[string]interface{})["eventType"]
		title := property.(map[string]interface{})["title"]
		level := property.(map[string]interface{})["level"]
		impactStartTime := property.(map[string]interface{})["impactStartTime"]

		common.HealthEvent.WithLabelValues(eventID.(string), title.(string), eventType.(string), lastUpdateTime.(string), impactStartTime.(string), status.(string), level.(string)).Inc()

		if status.(string) == "Resolved" {
			common.HealthClosedTotal.WithLabelValues(eventType.(string)).Inc()
		} else {
			common.HealthOpenedTotal.WithLabelValues(eventType.(string)).Inc()
		}
		impact := property.(map[string]interface{})["impact"]
		for _, imp := range impact.([]interface{}) {
			service := imp.(map[string]interface{})["impactedService"]
			regions := make([]string, 0)
			for _, region := range imp.(map[string]interface{})["impactedRegions"].([]interface{}) {
				regions = append(regions, region.(map[string]interface{})["impactedRegion"].(string))
			}
			common.AffectedEntity.WithLabelValues(eventID.(string), service.(string), strings.Join(regions, ",")).Inc()
		}
	}
	common.HealthEvent.Collect(ch)
	common.AffectedEntity.Collect(ch)
	common.HealthClosedTotal.Collect(ch)
	common.HealthOpenedTotal.Collect(ch)
}

func getToken(tenantId, clientId, clientSecret string) (string, error) {
	if ht, ok := http.DefaultTransport.(*http.Transport); ok {
		ht.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	url := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantId)
	scope := "https://management.azure.com/.default"

	postBody := []byte(fmt.Sprintf("client_id=%s&client_secret=%s&scope=%s&grant_type=client_credentials", clientId, clientSecret, scope))

	body := bytes.NewBuffer(postBody)

	resp, err := http.Post(url, "", body)
	if err != nil {
		log.Error("An Error Occured", err)
		return "", err
	}
	defer resp.Body.Close()
	respbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error("An Error Occurred", err)
		return "", err
	}
	obj := make(map[string]interface{})
	err = json.Unmarshal(respbody, &obj)
	if err != nil {
		log.Error("An Error Occurred", err)
		return "", err
	}
	if val, ok := obj["access_token"]; ok && val != nil {
		return val.(string), nil
	}
	return "", errors.New("can not get access_token")
}
