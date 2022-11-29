package alicloud

import (
	"context"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/quotas"
	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/common"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/constant"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
)

type QuotasClient interface {
	ListProductQuotas(request *quotas.ListProductQuotasRequest) (response *quotas.ListProductQuotasResponse, err error)
}

var ProdCodeList = []string{"ecs", "nat", "eip", "vpc", "slb", "ros"}

type MetricsCollectorAliQuota struct {
	conf   *config.Config
	log    log.FieldLogger
	client QuotasClient
}

type Result struct {
	quotaResult      *common.QuotaResult
	productId        string
	quotaDescription string
}

func NewMetricsCollectorAliQuota(config *config.Config, cred vault.CloudCredentials, logger log.FieldLogger) *MetricsCollectorAliQuota {
	client, err := quotas.NewClientWithAccessKey(config.Region, cred[vault.AliCloudAccessKeyID], cred[vault.AliCloudSecretAccessKey])
	if err != nil {
		logger.Errorf("Error while getting client: ", err)
	}
	m := &MetricsCollectorAliQuota{}
	m.conf = config
	m.log = logger
	m.log.Infof("Initialize AliCloud Quota client")
	m.client = client
	common.InitQuotaGaugeVec("AliCloud", []string{constant.LabelProductCode, constant.LabelQuotaName, constant.LabelQuotaCode, constant.LabelQuotaDescription, constant.LabelUnit})
	return m
}

func (m *MetricsCollectorAliQuota) Describe(ch chan<- *prometheus.Desc) {
	common.QuotaLimit.Describe(ch)
	common.QuotaCurrent.Describe(ch)
}

func (m *MetricsCollectorAliQuota) Collect(ch chan<- prometheus.Metric) {
	m.log.Infof("Start retrieve data from cache")
	for _, item := range common.QuotaCache.Items() {
		result := item.Object.(*Result)
		m.log.WithFields(log.Fields{"product": result.productId, "quotaName": result.quotaResult.QuotaName, "quotaCode": result.quotaResult.QuotaCode, "quotaDescription": result.quotaDescription, "current": result.quotaResult.CurrentValue, "limit": result.quotaResult.LimitValue}).Infof("retrieve data from cache")
		common.QuotaCurrent.WithLabelValues(result.productId, result.quotaResult.QuotaName, result.quotaResult.QuotaCode, result.quotaDescription, result.quotaResult.Unit).Set(result.quotaResult.CurrentValue)
		common.QuotaLimit.WithLabelValues(result.productId, result.quotaResult.QuotaName, result.quotaResult.QuotaCode, result.quotaDescription, result.quotaResult.Unit).Set(result.quotaResult.LimitValue)
	}
	common.QuotaLimit.Collect(ch)
	common.QuotaCurrent.Collect(ch)
}

func (m *MetricsCollectorAliQuota) scrape(ctx context.Context) {
	m.log.Infof("Start collect AliCloud quota metrics")
	for _, prod := range ProdCodeList {
		r := quotas.CreateListProductQuotasRequest()
		r.ProductCode = prod
		response, err := m.client.ListProductQuotas(r)
		if err != nil {
			m.log.Errorf("Error while traversing product resource list: ", err)
		}
		for _, quota := range response.Quotas {
			if quota.TotalUsage != 0 {
				quotaResult := &common.QuotaResult{QuotaName: quota.QuotaName, QuotaCode: quota.QuotaArn, LimitValue: quota.TotalQuota, CurrentValue: quota.TotalUsage, Unit: quota.QuotaUnit}
				result := &Result{quotaResult, prod, quota.QuotaDescription}
				common.QuotaCache.Set(quota.QuotaArn, result, cache.DefaultExpiration)
			}
		}
	}
	m.log.Infof("End collect AliCloud quota metrics")
}
