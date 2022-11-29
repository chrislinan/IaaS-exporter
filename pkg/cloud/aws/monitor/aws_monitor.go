package monitor

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConf "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwType "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	tagTypes "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
	"golang.org/x/exp/maps"
	"math"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"time"
)

const defaultLengthSeconds = int64(300)
const defaultPeriodSeconds = int32(300)

type cloudwatchData struct {
	ID                      *string
	MetricID                *string
	Metric                  *string
	Namespace               *string
	Statistics              []string
	GetMetricDataPoint      *float64
	GetMetricDataTimestamps *time.Time
	NilToZero               *bool
	AddCloudwatchTimestamp  *bool
	CustomTags              []config.Tag
	Tags                    []config.Tag
	Dimensions              []cwType.Dimension
	Region                  *string
	AccountId               *string
	Period                  int32
}

type MetricsCollectorAwsMonitor struct {
	log           log.FieldLogger
	conf          *config.Config
	accountId     string
	taggingClient *resourcegroupstaggingapi.Client
	cwClient      *cloudwatch.Client
	stsClient     *sts.Client
	metrics       []*PrometheusMetric
}

type dimValue2Res struct {
	dimVal string
	res    *taggedResource
}

func NewMetricsCollectorAwsMonitor(config *config.Config, cred vault.CloudCredentials, logger log.FieldLogger) *MetricsCollectorAwsMonitor {
	m := &MetricsCollectorAwsMonitor{}
	m.log = logger
	cfg, err := awsConf.LoadDefaultConfig(context.TODO(), awsConf.WithCredentialsProvider(credentials.StaticCredentialsProvider{Value: aws.Credentials{
		AccessKeyID:     cred[vault.AwsAccessKeyID],
		SecretAccessKey: cred[vault.AwsSecretAccessKey],
	}}), awsConf.WithRegion(config.Region))
	if err != nil {
		log.Fatal(err)
	}
	m.log.Infof("Start initialize tagging client")
	m.taggingClient = resourcegroupstaggingapi.NewFromConfig(cfg)
	m.log.Infof("Start initialize cloud watch client")
	m.cwClient = cloudwatch.NewFromConfig(cfg)
	m.log.Infof("Start initialize STS client")
	m.stsClient = sts.NewFromConfig(cfg)
	id, err := m.stsClient.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil {
		log.Fatal("Error while getting accountID: ", err)
	}
	m.accountId = aws.ToString(id.Account)
	m.conf = config
	return m
}

func (m *MetricsCollectorAwsMonitor) Describe(ch chan<- *prometheus.Desc) {
	for _, metric := range m.metrics {
		ch <- createDesc(metric)
	}
}

func (m *MetricsCollectorAwsMonitor) Collect(ch chan<- prometheus.Metric) {
	for _, metric := range m.metrics {
		ch <- createMetric(metric)
	}
}

func (m *MetricsCollectorAwsMonitor) collectData(ctx context.Context) []*cloudwatchData {
	wgJob := sync.WaitGroup{}
	mux := sync.Mutex{}
	cfg := m.conf.Aws.CloudWatchMetricsConf
	result := make([]*cloudwatchData, 0)
	for _, job := range cfg.Jobs {
		wgJob.Add(1)
		go func(job *config.Job) {
			m.log.Infof("Start collect data for job: %v", job.Type)
			defer wgJob.Done()
			var taggedRes []*taggedResource
			res := m.getTaggedResource(ctx, job, m.conf.Region)
			taggedRes = append(taggedRes, res...)
			svc := SupportedServices.GetService(job.Type)
			dimFilter := m.getDimensionsFilter(taggedRes, svc)
			wgMetric := sync.WaitGroup{}
			for _, metric := range job.Metrics {
				wgMetric.Add(1)
				m.log.Infof("Start collect data for job - metric: %v - %v", job.Type, metric.Name)
				go func(metric *config.Metric) {
					defer wgMetric.Done()
					m.log.Infof("Start collect full metrics list for %v, in namespace: %v", metric.Name, svc.Namespace)
					fullMetricsList := m.getFullMetricsListByName(ctx, aws.String(svc.Namespace), aws.String(metric.Name))
					filteredMetricsList := m.filterMetricsList(dimFilter, fullMetricsList)
					data := m.getCloudwatchDataFromMetric(dimFilter, filteredMetricsList, metric, job.Type, m.conf.Region, m.accountId, cfg.ExportedTagsOnMetrics, job.CustomTags)
					metricsData := m.scrapeDiscoveryJobUsingMetricData(ctx, svc, job, data)
					mux.Lock()
					result = append(result, metricsData...)
					mux.Unlock()
				}(metric)
			}
			wgMetric.Wait()
		}(job)
	}
	wgJob.Wait()
	return result
}

func (m *MetricsCollectorAwsMonitor) Scrape(ctx context.Context) {
	cwData := m.collectData(ctx)
	metrics, observedMetricLabels, err := createPrometheusMetricsFromCwData(cwData)
	metrics = ensureLabelConsistencyForMetrics(metrics, observedMetricLabels)
	if err != nil {
		m.log.Fatal(err)
	}
	m.metrics = metrics
}

func (m *MetricsCollectorAwsMonitor) scrapeDiscoveryJobUsingMetricData(ctx context.Context, svc *serviceFilter, job *config.Job, cwData []cloudwatchData) []*cloudwatchData {
	maxMetricCount := 20
	wg := sync.WaitGroup{}
	mux := &sync.Mutex{}
	var cw []*cloudwatchData
	length := getMetricDataInputLength(job)
	cutPoint := 0
	for cutPoint < len(cwData) {
		start := cutPoint
		end := cutPoint + maxMetricCount
		if end > len(cwData) {
			end = len(cwData)
		}
		wg.Add(1)
		go func(input []cloudwatchData) {
			defer wg.Done()
			filter := createGetMetricDataInput(input, &svc.Namespace, length, job.Delay, job.RoundingPeriod)
			data := &cloudwatch.GetMetricDataOutput{}
			paginator := cloudwatch.NewGetMetricDataPaginator(m.cwClient, filter)
			for paginator.HasMorePages() {
				page, err := paginator.NextPage(ctx)
				if err != nil {
					log.Fatal(err)
				}
				data.MetricDataResults = append(data.MetricDataResults, page.MetricDataResults...)
			}
			if data.MetricDataResults != nil {
				output := make([]*cloudwatchData, 0)
				for _, MetricDataResult := range data.MetricDataResults {
					getMetricData := findGetMetricDataById(input, *MetricDataResult.Id)
					if getMetricData != nil {
						if len(MetricDataResult.Values) != 0 {
							getMetricData.GetMetricDataPoint = &MetricDataResult.Values[0]
							getMetricData.GetMetricDataTimestamps = &MetricDataResult.Timestamps[0]
						}
						output = append(output, getMetricData)
					}
				}
				mux.Lock()
				cw = append(cw, output...)
				mux.Unlock()
			}
		}(cwData[start:end])
		cutPoint = end
	}
	wg.Wait()
	return cw
}

func (m *MetricsCollectorAwsMonitor) getTaggedResource(ctx context.Context, job *config.Job, region string) []*taggedResource {
	var resources []*taggedResource
	var wg sync.WaitGroup
	mux := &sync.Mutex{}
	m.log.Infof("Start getting tagged resources for job: %v in region: %v", job.Type, region)
	svc := SupportedServices.GetService(job.Type)
	if len(svc.ResourceFilters) > 0 {
		var tagFilters []tagTypes.TagFilter
		for _, tag := range job.SearchTags {
			tagFilters = append(tagFilters, tagTypes.TagFilter{
				Key: aws.String(tag.Key),
			})
		}
		input := &resourcegroupstaggingapi.GetResourcesInput{
			ResourceTypeFilters: svc.ResourceFilters,
			ResourcesPerPage:    aws.Int32(100),
			TagFilters:          tagFilters,
		}
		paginator := resourcegroupstaggingapi.NewGetResourcesPaginator(m.taggingClient, input)
		for paginator.HasMorePages() {
			wg.Add(1)
			page, err := paginator.NextPage(ctx)
			if err != nil {
				log.Fatal(err)
			}
			go func(out *resourcegroupstaggingapi.GetResourcesOutput) {
				defer wg.Done()
				for _, resourceTagMapping := range out.ResourceTagMappingList {
					resource := taggedResource{
						ARN:       aws.ToString(resourceTagMapping.ResourceARN),
						Namespace: job.Type,
						Region:    region,
					}
					for _, t := range resourceTagMapping.Tags {
						resource.Tags = append(resource.Tags, config.Tag{Key: *t.Key, Value: *t.Value})
					}
					if resource.filterThroughTags(job.SearchTags) {
						mux.Lock()
						resources = append(resources, &resource)
						mux.Unlock()
					}
				}
			}(page)
		}
		wg.Wait()
	}
	m.log.Infof("%v tagged resources collected", len(resources))
	return resources
}

func (m *MetricsCollectorAwsMonitor) getDimensionsFilter(taggedRes []*taggedResource, svc *serviceFilter) map[string][]dimValue2Res {
	dimensionsFilter := make(map[string][]dimValue2Res)
	for _, dr := range svc.DimensionRegexps {
		dimensionRegexp := regexp.MustCompile(*dr)
		names := dimensionRegexp.SubexpNames()
		for _, dimensionName := range names[1:] {
			if _, ok := dimensionsFilter[dimensionName]; !ok {
				dimensionsFilter[dimensionName] = []dimValue2Res{}
			}
		}
		for _, r := range taggedRes {
			if dimensionRegexp.Match([]byte(r.ARN)) {
				dimensionMatch := dimensionRegexp.FindStringSubmatch(r.ARN)
				for i, value := range dimensionMatch {
					if i == 0 {
						continue
					}
					dimensionsFilter[names[i]] = append(dimensionsFilter[names[i]], dimValue2Res{value, r})
				}
			}
		}
	}
	return dimensionsFilter
}

func (m *MetricsCollectorAwsMonitor) getFullMetricsListByName(ctx context.Context, namespace, metricsName *string) []cwType.Metric {
	var output []cwType.Metric
	input := &cloudwatch.ListMetricsInput{
		MetricName: metricsName,
		Namespace:  namespace,
	}
	paginator := cloudwatch.NewListMetricsPaginator(m.cwClient, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			log.Fatal(err)
		}
		output = append(output, page.Metrics...)
	}
	m.log.Infof("%v records collected for metric: %v in namespace: %v", len(output), aws.ToString(metricsName), aws.ToString(namespace))
	return output
}

func (m *MetricsCollectorAwsMonitor) filterMetricsList(dimFilter map[string][]dimValue2Res, fullMetricsList []cwType.Metric) []cwType.Metric {
	var result []cwType.Metric
	wg := sync.WaitGroup{}
	mux := &sync.Mutex{}
	cutPoint := 0
	m.log.Infof("Start filter metrics list with dim: %v", maps.Keys(dimFilter))
	for cutPoint < len(fullMetricsList) {
		start := cutPoint
		end := cutPoint + 50
		if end > len(fullMetricsList) {
			end = len(fullMetricsList)
		}
		wg.Add(1)
		go func(dimFilter map[string][]dimValue2Res, metricList []cwType.Metric) {
			defer wg.Done()
			for _, metric := range metricList {
				for _, d := range metric.Dimensions {
					if _, ok := dimFilter[aws.ToString(d.Name)]; ok {
						for _, val := range dimFilter[aws.ToString(d.Name)] {
							if val.dimVal == aws.ToString(d.Value) {
								mux.Lock()
								result = append(result, metric)
								mux.Unlock()
							}
						}
					}
				}
			}
		}(dimFilter, fullMetricsList[start:end])
		cutPoint = end
	}
	wg.Wait()
	m.log.Infof("%v metrics discovered after filter", len(result))
	return result
}

func (m *MetricsCollectorAwsMonitor) getCloudwatchDataFromMetric(dimFilter map[string][]dimValue2Res, filteredMetricsList []cwType.Metric, metric *config.Metric, namespace, region, accountId string, tagsOnMetrics config.ExportedTagsOnMetrics, customTags []config.Tag) []cloudwatchData {
	var result []cloudwatchData
	var r *taggedResource
	for _, cwMetric := range filteredMetricsList {
		found := false
		for _, dimension := range cwMetric.Dimensions {
			if found {
				break
			}
			for _, val := range dimFilter[aws.ToString(dimension.Name)] {
				if val.dimVal == aws.ToString(dimension.Value) {
					r = val.res
					found = true
					break
				}
			}
		}

		for _, stats := range metric.Statistics {
			id := fmt.Sprintf("id_%d", rand.Int())
			metricTags := r.metricTags(tagsOnMetrics)
			d := cloudwatchData{
				ID:                     &r.ARN,
				MetricID:               &id,
				Metric:                 &metric.Name,
				Namespace:              &namespace,
				Statistics:             []string{stats},
				NilToZero:              metric.NilToZero,
				AddCloudwatchTimestamp: metric.AddCloudwatchTimestamp,
				Tags:                   metricTags,
				CustomTags:             customTags,
				Dimensions:             cwMetric.Dimensions,
				Region:                 &region,
				AccountId:              &accountId,
				Period:                 metric.Period,
			}
			result = append(result, d)
		}
	}
	return result
}

func findGetMetricDataById(getMetricData []cloudwatchData, value string) *cloudwatchData {
	for _, data := range getMetricData {
		if *data.MetricID == value {
			return &data
		}
	}
	return nil
}

func getMetricDataInputLength(job *config.Job) int64 {
	length := defaultLengthSeconds

	if job.Length > 0 {
		length = job.Length
	}
	for _, metric := range job.Metrics {
		if metric.Length > length {
			length = metric.Length
		}
	}
	return length
}

func createGetMetricDataInput(getMetricData []cloudwatchData, namespace *string, length int64, delay int64, configuredRoundingPeriod *int32) (output *cloudwatch.GetMetricDataInput) {
	var metricsDataQuery []cwType.MetricDataQuery
	wg := sync.WaitGroup{}
	mux := &sync.Mutex{}
	cutPoint := 0
	roundingPeriod := defaultPeriodSeconds
	for cutPoint < len(getMetricData) {
		start := cutPoint
		end := cutPoint + 50
		if end > len(getMetricData) {
			end = len(getMetricData)
		}
		wg.Add(1)
		go func(input []cloudwatchData) {
			defer wg.Done()
			for _, data := range input {
				if data.Period < roundingPeriod {
					roundingPeriod = data.Period
				}
				metricStat := &cwType.MetricStat{
					Metric: &cwType.Metric{
						Dimensions: data.Dimensions,
						MetricName: data.Metric,
						Namespace:  namespace,
					},
					Period: &data.Period,
					Stat:   &data.Statistics[0],
				}
				ReturnData := true
				mux.Lock()
				metricsDataQuery = append(metricsDataQuery, cwType.MetricDataQuery{
					Id:         data.MetricID,
					MetricStat: metricStat,
					ReturnData: &ReturnData,
				})
				mux.Unlock()
			}
		}(getMetricData[start:end])
		cutPoint = end
	}
	wg.Wait()

	if configuredRoundingPeriod != nil {
		roundingPeriod = *configuredRoundingPeriod
	}

	startTime, endTime := determineGetMetricDataWindow(
		time.Now(),
		time.Duration(roundingPeriod)*time.Second,
		time.Duration(length)*time.Second,
		time.Duration(delay)*time.Second,
	)

	output = &cloudwatch.GetMetricDataInput{
		EndTime:           &endTime,
		StartTime:         &startTime,
		MetricDataQueries: metricsDataQuery,
		ScanBy:            "TimestampDescending",
	}
	return output
}

func determineGetMetricDataWindow(now time.Time, roundingPeriod time.Duration, length time.Duration, delay time.Duration) (time.Time, time.Time) {
	if roundingPeriod > 0 {
		// Round down the time to a factor of the period - rounding is recommended by AWS:
		// https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_GetMetricData.html#API_GetMetricData_RequestParameters
		now = now.Add(-roundingPeriod / 2).Round(roundingPeriod)
	}
	startTime := now.Add(-(length + delay)).UTC()
	endTime := now.Add(-delay).UTC()
	return startTime, endTime
}

func createPrometheusMetricsFromCwData(cwd []*cloudwatchData) ([]*PrometheusMetric, map[string]LabelSet, error) {
	output := make([]*PrometheusMetric, 0)
	observedMetricLabels := make(map[string]LabelSet)
	for _, c := range cwd {
		for _, statistic := range c.Statistics {
			var includeTimestamp bool
			if c.AddCloudwatchTimestamp != nil {
				includeTimestamp = *c.AddCloudwatchTimestamp
			}
			var exportedDatapoint *float64
			var timestamp time.Time
			if c.GetMetricDataPoint != nil {
				exportedDatapoint, timestamp = c.GetMetricDataPoint, *c.GetMetricDataTimestamps
			}
			if exportedDatapoint == nil && (c.AddCloudwatchTimestamp == nil || !*c.AddCloudwatchTimestamp) {
				var nan = math.NaN()
				exportedDatapoint = &nan
				includeTimestamp = false
				//if *c.NilToZero {
				//	var zero float64 = 0
				//	exportedDatapoint = &zero
				//}
			}
			promNs := strings.ToLower(*c.Namespace)
			if !strings.HasPrefix(promNs, "aws") {
				promNs = "cpe_aws_" + promNs
			}
			name := promString(promNs) + "_" + strings.ToLower(promString(*c.Metric)) + "_" + strings.ToLower(promString(statistic))
			if exportedDatapoint != nil {
				promLabels := createPrometheusLabels(c)
				observedMetricLabels = recordLabelsForMetric(name, promLabels, observedMetricLabels)
				p := PrometheusMetric{
					name:             &name,
					labels:           promLabels,
					value:            exportedDatapoint,
					timestamp:        timestamp,
					includeTimestamp: includeTimestamp,
				}
				output = append(output, &p)
			}
		}
	}
	return output, observedMetricLabels, nil
}

func ensureLabelConsistencyForMetrics(metrics []*PrometheusMetric, observedMetricLabels map[string]LabelSet) []*PrometheusMetric {
	for _, prometheusMetric := range metrics {
		for observedLabel := range observedMetricLabels[*prometheusMetric.name] {
			if _, ok := prometheusMetric.labels[observedLabel]; !ok {
				prometheusMetric.labels[observedLabel] = ""
			}
		}
	}
	return metrics
}
