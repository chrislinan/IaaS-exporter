package health

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConf "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/health"
	"github.com/aws/aws-sdk-go-v2/service/health/types"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/common"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
)

type IHealthClient interface {
	DescribeEvents(context.Context, *health.DescribeEventsInput, ...func(*health.Options)) (*health.DescribeEventsOutput, error)
	DescribeAffectedEntities(context.Context, *health.DescribeAffectedEntitiesInput, ...func(*health.Options)) (*health.DescribeAffectedEntitiesOutput, error)
}

type MetricsCollectorAwsHealth struct {
	conf         *config.Config
	healthClient IHealthClient
	log          log.FieldLogger
}

func NewMetricsCollectorAwsHealth(config *config.Config, cred vault.CloudCredentials, logger log.FieldLogger) *MetricsCollectorAwsHealth {
	m := &MetricsCollectorAwsHealth{}
	m.conf = config
	m.log = logger
	cfg, err := awsConf.LoadDefaultConfig(context.TODO(), awsConf.WithCredentialsProvider(credentials.StaticCredentialsProvider{Value: aws.Credentials{
		AccessKeyID:     cred[vault.AwsAccessKeyID],
		SecretAccessKey: cred[vault.AwsSecretAccessKey],
	}}), awsConf.WithRegion(config.Region))
	if err != nil {
		m.log.Fatal(err)
	}
	m.healthClient = health.NewFromConfig(cfg)
	eventLabel := []string{
		"eventID",
		"cloudService",
		"eventRegion",
		"startTime",
		"statusCode",
		"lastUpdatedTime",
		"eventTypeCode",
		"eventType",
		"eventScopeCode",
		"availabilityZone",
	}

	entityLabel := []string{
		"eventID",
		"accountID",
		"affectedRegions",
		"entityArn",
		"statusCode",
		"entityValue",
		"entityUrl",
		"lastUpdatedTime",
	}

	openTotalLabel := []string{"eventType", "availabilityZone", "cloudService"}
	closeTotalLabel := []string{"eventType", "availabilityZone", "cloudService"}
	common.InitHealthCounterVec("AWS", eventLabel, entityLabel, openTotalLabel, closeTotalLabel)
	return m
}

func (m *MetricsCollectorAwsHealth) Describe(ch chan<- *prometheus.Desc) {
	common.HealthEvent.Describe(ch)
	common.AffectedEntity.Describe(ch)
	common.HealthOpenedTotal.Describe(ch)
	common.HealthClosedTotal.Describe(ch)
}

func (m *MetricsCollectorAwsHealth) Collect(ch chan<- prometheus.Metric) {
	common.HealthOpenedTotal.Reset()
	common.HealthClosedTotal.Reset()
	common.HealthEvent.Reset()
	common.AffectedEntity.Reset()
	var eventArn [][]*string
	var events []types.Event
	var HealthEventStatusCodes []types.EventStatusCode
	for _, EventStatusCode := range m.conf.Aws.HealthEventStatusCodes {
		HealthEventStatusCodes = append(HealthEventStatusCodes, types.EventStatusCode(EventStatusCode))
	}
	var HealthEventTypeCategories []types.EventTypeCategory
	for _, EventTypeCategory := range m.conf.Aws.HealthEventTypeCategories {
		HealthEventTypeCategories = append(HealthEventTypeCategories, types.EventTypeCategory(EventTypeCategory))
	}
	eventFilter := &types.EventFilter{EventStatusCodes: HealthEventStatusCodes, EventTypeCategories: HealthEventTypeCategories} // closed, open, upcoming
	eventParams := &health.DescribeEventsInput{Filter: eventFilter}
	eventPaginator := health.NewDescribeEventsPaginator(m.healthClient, eventParams)
	for eventPaginator.HasMorePages() {
		output, err := eventPaginator.NextPage(context.TODO())
		if err != nil {
			m.log.Fatal(err)
		}
		events = append(events, output.Events...)
	}

	m.log.Infof("The number of total events: ", len(events))
	if len(events) == 0 {
		return
	}

	var arnList []*string
	regionMap := make(map[string]string)
	for _, event := range events {
		common.HealthEvent.WithLabelValues(aws.ToString(event.Arn), aws.ToString(event.Service), aws.ToString(event.Region), aws.ToTime(event.StartTime).String(),
			string(event.StatusCode), aws.ToTime(event.LastUpdatedTime).String(), aws.ToString(event.EventTypeCode),
			string(event.EventTypeCategory), string(event.EventScopeCode), aws.ToString(event.AvailabilityZone)).Inc()
		regionMap[aws.ToString(event.Arn)] = aws.ToString(event.Region)

		if event.StatusCode == types.EventStatusCodeClosed {
			common.HealthClosedTotal.WithLabelValues(string(event.EventTypeCategory), aws.ToString(event.AvailabilityZone), aws.ToString(event.Service)).Inc()
		} else if event.StatusCode == types.EventStatusCodeOpen {
			common.HealthOpenedTotal.WithLabelValues(string(event.EventTypeCategory), aws.ToString(event.AvailabilityZone), aws.ToString(event.Service)).Inc()
		}
		arnList = append(arnList, event.Arn)
		if len(arnList) == 10 {
			eventArn = append(eventArn, arnList)
			arnList = []*string{}
		}
	}
	if len(arnList) != 0 {
		eventArn = append(eventArn, arnList)
	}

	var entities []types.AffectedEntity

	for _, arn := range eventArn {
		entities = entities[:0]
		entityFilter := &types.EntityFilter{EventArns: aws.ToStringSlice(arn)}
		entityParams := &health.DescribeAffectedEntitiesInput{Filter: entityFilter}
		entityPaginator := health.NewDescribeAffectedEntitiesPaginator(m.healthClient, entityParams)
		for entityPaginator.HasMorePages() {
			output, err := entityPaginator.NextPage(context.TODO())
			if err != nil {
				m.log.Fatal(err)
			}
			entities = append(entities, output.Entities...)
		}
		for _, entity := range entities {
			common.AffectedEntity.WithLabelValues(aws.ToString(entity.EventArn), aws.ToString(entity.AwsAccountId), regionMap[aws.ToString(entity.EventArn)], aws.ToString(entity.EntityArn),
				string(entity.StatusCode), aws.ToString(entity.EntityValue), aws.ToString(entity.EntityUrl), aws.ToTime(entity.LastUpdatedTime).String()).Inc()
		}
	}
	common.HealthEvent.Collect(ch)
	common.AffectedEntity.Collect(ch)
	common.HealthOpenedTotal.Collect(ch)
	common.HealthClosedTotal.Collect(ch)
}
