package alicloud

import (
	"context"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/quotas"
	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/common"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
	"net/http"
	"testing"
	"time"
)

type MockQuotasClient struct {
}

type MockQuotaCache struct {
}

func (m *MockQuotaCache) Set(k string, x interface{}, d time.Duration) {}

func (m *MockQuotaCache) Items() map[string]cache.Item {
	var result = make(map[string]cache.Item)
	for _, prod := range ProdCodeList {
		result[prod] = cache.Item{
			Expiration: 0,
			Object: &Result{
				productId:        prod,
				quotaDescription: "desc",
				quotaResult: &common.QuotaResult{
					QuotaCode:    "code",
					QuotaName:    "name",
					LimitValue:   100,
					CurrentValue: 30,
					Unit:         "unit",
				},
			},
		}
	}
	return result
}

func (m *MockQuotasClient) ListProductQuotas(request *quotas.ListProductQuotasRequest) (response *quotas.ListProductQuotasResponse, err error) {
	var quota []quotas.QuotasItemInListProductQuotas
	quota = append(quota, quotas.QuotasItemInListProductQuotas{
		TotalQuota:       float64(200),
		TotalUsage:       float64(160),
		QuotaName:        "dummy_name",
		QuotaUnit:        "dummy_unit",
		QuotaDescription: "dummy_description",
		QuotaType:        "dummy_type",
	})
	result := &quotas.ListProductQuotasResponse{Quotas: quota}
	return result, nil
}

func TestAliCloudQuota(t *testing.T) {
	uri := "/metrics"
	conf := &config.Config{}
	cred := vault.CloudCredentials{
		vault.AliCloudAccessKeyID:     "AliCloudAccessKeyID",
		vault.AliCloudSecretAccessKey: "AliCloudSecretAccessKey",
	}
	quotaCollector := NewMetricsCollectorAliQuota(conf, cred, &log.Logger{})
	quotaCollector.client = &MockQuotasClient{}
	common.QuotaCache = &MockQuotaCache{}
	registry := prometheus.NewRegistry()
	registry.MustRegister(quotaCollector)
	quotaCollector.scrape(context.TODO())
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	})

	for _, prod := range ProdCodeList {
		assert.HTTPBodyContains(t, handler, "GET", uri, nil, "cpe_quota_current{ProductCode=\""+prod+"\",QuotaCode=\"code\",QuotaDescription=\"desc\",QuotaName=\"name\",Unit=\"unit\"} 30")
		assert.HTTPBodyContains(t, handler, "GET", uri, nil, "cpe_quota_limit{ProductCode=\""+prod+"\",QuotaCode=\"code\",QuotaDescription=\"desc\",QuotaName=\"name\",Unit=\"unit\"} 100")
	}
}
