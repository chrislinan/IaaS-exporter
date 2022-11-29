package health

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/health"
	"github.com/aws/aws-sdk-go-v2/service/health/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
	"net/http"
	"testing"
	"time"
)

type MockHealthClient struct {
}

var t = time.Unix(1469581066, 0).UTC()

func (m *MockHealthClient) DescribeAffectedEntities(ctx context.Context, input *health.DescribeAffectedEntitiesInput, f ...func(*health.Options)) (*health.DescribeAffectedEntitiesOutput, error) {
	output := &health.DescribeAffectedEntitiesOutput{
		Entities: []types.AffectedEntity{
			{
				AwsAccountId:    aws.String("dummyAccountID"),
				EntityArn:       aws.String("dummyEntityArn"),
				EntityUrl:       aws.String("dummyEntityUrl"),
				EntityValue:     aws.String("dummyValue"),
				EventArn:        aws.String("dummyEventArn"),
				LastUpdatedTime: aws.Time(t),
				StatusCode:      types.EntityStatusCodeImpaired,
			},
		},
	}
	return output, nil
}

func (m *MockHealthClient) DescribeEvents(context.Context, *health.DescribeEventsInput, ...func(*health.Options)) (*health.DescribeEventsOutput, error) {
	output := &health.DescribeEventsOutput{
		Events: []types.Event{
			{
				Arn:               aws.String("dummyArn"),
				AvailabilityZone:  aws.String("dummyAZ"),
				EventScopeCode:    types.EventScopeCodeNone,
				EventTypeCategory: types.EventTypeCategoryIssue,
				EventTypeCode:     aws.String("dummyEventTypeCode"),
				Region:            aws.String("dummyRegion"),
				Service:           aws.String("dummyService"),
				StatusCode:        types.EventStatusCodeOpen,
				StartTime:         aws.Time(t),
				EndTime:           aws.Time(t),
				LastUpdatedTime:   aws.Time(t),
			},
			{
				Arn:               aws.String("dummyArn1"),
				AvailabilityZone:  aws.String("dummyAZ1"),
				EventScopeCode:    types.EventScopeCodeNone,
				EventTypeCategory: types.EventTypeCategoryIssue,
				EventTypeCode:     aws.String("dummyEventTypeCode1"),
				Region:            aws.String("dummyRegion1"),
				Service:           aws.String("dummyService1"),
				StatusCode:        types.EventStatusCodeClosed,
				StartTime:         aws.Time(t),
				EndTime:           aws.Time(t),
				LastUpdatedTime:   aws.Time(t),
			},
			{
				Arn:               aws.String("dummyArn2"),
				AvailabilityZone:  aws.String("dummyAZ1"),
				EventScopeCode:    types.EventScopeCodeNone,
				EventTypeCategory: types.EventTypeCategoryIssue,
				EventTypeCode:     aws.String("dummyEventTypeCode1"),
				Region:            aws.String("dummyRegion2"),
				Service:           aws.String("dummyService1"),
				StatusCode:        types.EventStatusCodeClosed,
				StartTime:         aws.Time(t),
				EndTime:           aws.Time(t),
				LastUpdatedTime:   aws.Time(t),
			},
		},
	}
	return output, nil
}

func TestAwsHealth(t *testing.T) {
	uri := "/metrics"
	cred := vault.CloudCredentials{
		vault.AwsAccessKeyID:     "accessKeyID",
		vault.AwsSecretAccessKey: "secretAccessKey",
	}
	conf := &config.Config{
		Region: "eu-central-1",
		Aws:    &config.AwsConfig{HealthEventStatusCodes: []string{"open", "closed"}},
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		healthCollector := NewMetricsCollectorAwsHealth(conf, cred, &log.Logger{})
		healthCollector.healthClient = &MockHealthClient{}
		registry := prometheus.NewRegistry()
		registry.MustRegister(healthCollector)
		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	})

	assert.HTTPBodyContains(t, handler, "GET", uri, nil, "cpe_health_events{availabilityZone=\"dummyAZ\",cloudService=\"dummyService\",eventID=\"dummyArn\",eventRegion=\"dummyRegion\",eventScopeCode=\"NONE\",eventType=\"issue\",eventTypeCode=\"dummyEventTypeCode\",lastUpdatedTime=\"2016-07-27 00:57:46 +0000 UTC\",startTime=\"2016-07-27 00:57:46 +0000 UTC\",statusCode=\"open\"} 1")
	assert.HTTPBodyContains(t, handler, "GET", uri, nil, "cpe_health_events_affected{accountID=\"dummyAccountID\",affectedRegions=\"\",entityArn=\"dummyEntityArn\",entityUrl=\"dummyEntityUrl\",entityValue=\"dummyValue\",eventID=\"dummyEventArn\",lastUpdatedTime=\"2016-07-27 00:57:46 +0000 UTC\",statusCode=\"IMPAIRED\"} 1")
	assert.HTTPBodyContains(t, handler, "GET", uri, nil, "cpe_health_events_opened_total{availabilityZone=\"dummyAZ\",cloudService=\"dummyService\",eventType=\"issue\"} 1")
	assert.HTTPBodyContains(t, handler, "GET", uri, nil, "cpe_health_events_closed_total{availabilityZone=\"dummyAZ1\",cloudService=\"dummyService1\",eventType=\"issue\"} 2")
}
