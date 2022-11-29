package azure

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/network/mgmt/network"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/storage/mgmt/storage"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
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

type MockStorageClient struct {
}

type MockNetworkClient struct {
}

type MockComputeClient struct {
}

type MockSubscriptionInfo struct {
}

func (m *MockStorageClient) SetAuth(auth autorest.Authorizer) {
}

func (m *MockNetworkClient) SetAuth(auth autorest.Authorizer) {
}

func (m *MockComputeClient) SetAuth(auth autorest.Authorizer) {
}

type MockQuotaCache struct {
}

func (m *MockQuotaCache) Set(k string, x interface{}, d time.Duration) {}

func (m *MockQuotaCache) Items() map[string]cache.Item {
	var result = map[string]cache.Item{
		"1": {
			Expiration: 0,
			Object: &common.QuotaResult{
				QuotaCode:    "code",
				QuotaName:    "name",
				LimitValue:   100,
				CurrentValue: 30,
			},
		},
	}
	return result
}

func (m *MockStorageClient) ListByLocation(ctx context.Context, location string) (result storage.UsageListResult, err error) {
	var value []storage.Usage
	value = append(value, storage.Usage{
		Unit:         "UsageUnitCount",
		CurrentValue: to.Int32Ptr(50),
		Limit:        to.Int32Ptr(100),
		Name: &storage.UsageName{
			Value:          to.StringPtr("dummy_value"),
			LocalizedValue: to.StringPtr("dummy_storage"),
		},
	})
	output := storage.UsageListResult{Value: &value}
	return output, nil
}

func (m *MockNetworkClient) ListComplete(ctx context.Context, location string) (result network.UsagesListResultIterator, err error) {
	var value []network.Usage
	value = append(value, network.Usage{
		Unit:         to.StringPtr("UsageUnitCount"),
		CurrentValue: to.Int64Ptr(60),
		Limit:        to.Int64Ptr(100),
		Name: &network.UsageName{
			Value:          to.StringPtr("dummy_value"),
			LocalizedValue: to.StringPtr("dummy_network"),
		},
	})
	r := network.UsagesListResult{
		Value:    &value,
		NextLink: nil,
	}
	page := network.NewUsagesListResultPage(r, func(ctx context.Context, result network.UsagesListResult) (network.UsagesListResult, error) {
		return network.UsagesListResult{}, nil
	})

	outputs := network.NewUsagesListResultIterator(page)
	return outputs, nil
}

func (m *MockComputeClient) ListComplete(ctx context.Context, location string) (result compute.ListUsagesResultIterator, err error) {
	var value []compute.Usage
	value = append(value, compute.Usage{
		Unit:         to.StringPtr("UsageUnitCount"),
		CurrentValue: to.Int32Ptr(70),
		Limit:        to.Int64Ptr(100),
		Name: &compute.UsageName{
			Value:          to.StringPtr("dummy_value"),
			LocalizedValue: to.StringPtr("dummy_compute"),
		},
	})
	r := compute.ListUsagesResult{
		Value:    &value,
		NextLink: nil,
	}
	page := compute.NewListUsagesResultPage(r, func(ctx context.Context, result compute.ListUsagesResult) (compute.ListUsagesResult, error) {
		return compute.ListUsagesResult{}, nil
	})

	outputs := compute.NewListUsagesResultIterator(page)
	return outputs, nil
}

func (m *MockSubscriptionInfo) GetSubscriptionInfo(logger log.FieldLogger) (subscriptionID, subscriptionName string) {
	return "mock_subscriptionID", "mock_subscriptionName"
}

func TestAzureQuota(t *testing.T) {
	uri := "/metrics"
	cred := vault.CloudCredentials{
		vault.AzureClientSecret:   "clientSecret",
		vault.AzureClientID:       "clientID",
		vault.AzureTenantID:       "tenantID",
		vault.AzureSubscriptionID: "subscriptionID",
	}

	vaultBackupBucket := config.VaultBackupBucketConfig{
		Bucket: "mock_bucket",
		Prefix: "mock_prefix",
	}
	conf := &config.Config{
		VaultBackupBucket: &vaultBackupBucket,
		Region:            "dummy_region",
		Azure: &config.AzureConfig{
			SubscriptionID: "a68ae472-1849-4ed9-a700-24f5070acd2d",
		},
	}

	common.QuotaCache = &MockQuotaCache{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		quotaCollector := NewMetricsCollectorAzureRmQuota(conf, cred, &log.Logger{})
		quotaCollector.storageUsageClient = &MockStorageClient{}
		quotaCollector.networkUsageClient = &MockNetworkClient{}
		quotaCollector.computeUsageClient = &MockComputeClient{}
		quotaCollector.subscriptionInfo = &MockSubscriptionInfo{}
		quotaCollector.scrape(context.TODO())
		registry := prometheus.NewRegistry()
		registry.MustRegister(quotaCollector)
		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	})

	assert.HTTPBodyContains(t, handler, "GET", uri, nil, "cpe_quota_current{QuotaCode=\"code\",QuotaName=\"name\",Region=\"dummy_region\",SubscriptionID=\"mock_subscriptionID\",SubscriptionName=\"mock_subscriptionName\",Unit=\"\"} 30")
	assert.HTTPBodyContains(t, handler, "GET", uri, nil, "cpe_quota_limit{QuotaCode=\"code\",QuotaName=\"name\",Region=\"dummy_region\",SubscriptionID=\"mock_subscriptionID\",SubscriptionName=\"mock_subscriptionName\",Unit=\"\"} 100")
}
