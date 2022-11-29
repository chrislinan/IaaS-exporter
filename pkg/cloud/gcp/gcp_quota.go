package gcp

import (
	"context"
	"fmt"
	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/common"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/constant"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	"strings"
	"time"
)

type ServiceClient interface {
	GetProject(project string) (*compute.Project, error)
	GetRegionList(project string) (*compute.RegionList, error)
}

type ServiceClientWrapper struct {
	client *compute.Service
}

func (cw *ServiceClientWrapper) GetProject(project string) (*compute.Project, error) {
	return cw.client.Projects.Get(project).Do()
}

func (cw *ServiceClientWrapper) GetRegionList(project string) (*compute.RegionList, error) {
	return cw.client.Regions.List(project).Do()
}

func NewServiceClientWrapper(creds *google.Credentials, logger log.FieldLogger) *ServiceClientWrapper {
	c, err := compute.NewService(context.Background(), option.WithCredentials(creds))
	if err != nil {
		logger.Errorf("Error while getting service client: ", err)
	}
	cw := &ServiceClientWrapper{client: c}
	return cw
}

type MetricsCollectorGcpRmQuota struct {
	conf    *config.Config
	client  ServiceClient
	project string
	log     log.FieldLogger
}

type Result struct {
	quotaResult *common.QuotaResult
	region      string
	project     *compute.Project
	regional    bool
}

func (m *MetricsCollectorGcpRmQuota) Describe(ch chan<- *prometheus.Desc) {
	common.QuotaLimit.Describe(ch)
	common.QuotaCurrent.Describe(ch)
}

func (m *MetricsCollectorGcpRmQuota) scrape(ctx context.Context) {
	m.log.Infof("Start collect GCP metrics")
	m.log.Infof("Start collect GCP project metrics")
	project, err := m.client.GetProject(m.project)
	if err != nil {
		m.log.Errorf("Error while getting project: ", err)
	}
	for _, quota := range project.Quotas {
		if quota.Usage != 0 {
			quotaResult := &common.QuotaResult{QuotaCode: quota.Metric, QuotaName: strings.ReplaceAll(quota.Metric, "_", " "), LimitValue: quota.Limit, CurrentValue: quota.Usage}
			result := &Result{project: project, quotaResult: quotaResult, regional: false, region: m.conf.Region}
			common.QuotaCache.Set(quota.Metric, result, cache.DefaultExpiration)

		}
	}
	m.log.Infof("Start collect GCP regional metrics")
	regionList, err := m.client.GetRegionList(m.project)
	if err != nil {
		m.log.Errorf("Error while getting region list: ", err)
	}
	for _, region := range regionList.Items {
		for _, quota := range region.Quotas {
			if quota.Usage != 0 {
				quotaResult := &common.QuotaResult{QuotaCode: quota.Metric, QuotaName: strings.ReplaceAll(quota.Metric, "_", " "), LimitValue: quota.Limit, CurrentValue: quota.Usage}
				result := &Result{project: project, quotaResult: quotaResult, regional: true, region: m.conf.Region}
				common.QuotaCache.Set(quota.Metric, result, cache.DefaultExpiration)
			}
		}
	}
	m.log.Infof("End collect GCP metrics")
}

func (m *MetricsCollectorGcpRmQuota) Collect(ch chan<- prometheus.Metric) {
	m.log.Infof("Start retrieve data from cache")
	for _, item := range common.QuotaCache.Items() {
		result := item.Object.(*Result)
		m.log.WithFields(log.Fields{"regional": result.regional, "region": result.region, "quotaCode": result.quotaResult.QuotaCode, "quotaName": result.quotaResult.QuotaName, "projectId": result.project.Id, "projectName": result.project.Name, "current": result.quotaResult.CurrentValue, "limit": result.quotaResult.LimitValue}).Infof("retrieve data from cache")
		common.QuotaCurrent.WithLabelValues(fmt.Sprintf("%v", result.regional), result.region, result.quotaResult.QuotaCode, result.quotaResult.QuotaName, fmt.Sprintf("%d", result.project.Id), result.project.Name).Set(result.quotaResult.CurrentValue)
		common.QuotaLimit.WithLabelValues(fmt.Sprintf("%v", result.regional), result.region, result.quotaResult.QuotaCode, result.quotaResult.QuotaName, fmt.Sprintf("%d", result.project.Id), result.project.Name).Set(result.quotaResult.LimitValue)
	}
	common.QuotaLimit.Collect(ch)
	common.QuotaCurrent.Collect(ch)
}

func NewMetricsCollectorGcpRmQuota(config *config.Config, cred vault.CloudCredentials, logger log.FieldLogger) *MetricsCollectorGcpRmQuota {
	m := &MetricsCollectorGcpRmQuota{}
	m.conf = config
	m.log = logger
	credential, err := google.CredentialsFromJSON(context.Background(), []byte(cred[vault.GcpServiceAccount]), constant.GCPQuotaScope)
	if err != nil {
		m.log.Errorf("Error while getting credential: ", err)
	}

	c := NewServiceClientWrapper(credential, m.log)
	m.client = c
	m.project = cred[vault.GcpProjectID]

	common.QuotaCache = cache.New(6*time.Minute, 10*time.Minute)
	common.InitQuotaGaugeVec("GCP", []string{constant.LabelRegional, constant.LabelRegion, constant.LabelQuotaCode, constant.LabelQuotaName, constant.LabelProjectID, constant.LabelProjectName})
	return m
}
