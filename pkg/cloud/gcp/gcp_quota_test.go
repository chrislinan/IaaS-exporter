package gcp

import (
	"context"
	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/common"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
	"google.golang.org/api/compute/v1"
	"net/http"
	"testing"
	"time"
)

type MockServiceClient struct {
}

func (m *MockServiceClient) GetProject(project string) (*compute.Project, error) {
	var quotas []*compute.Quota
	quotas = append(quotas, &compute.Quota{
		Usage:  float64(50),
		Limit:  float64(100),
		Metric: "CPUS_ALL_REGIONS",
	})
	projects := &compute.Project{
		Quotas: quotas,
		Id:     uint64(001),
		Name:   "mock_project",
	}
	return projects, nil
}

func (m *MockServiceClient) GetRegionList(project string) (*compute.RegionList, error) {
	var quotas []*compute.Quota
	quotas = append(quotas, &compute.Quota{
		Usage:  float64(80),
		Limit:  float64(100),
		Metric: "CPUS",
	})
	var regions []*compute.Region
	regions = append(regions, &compute.Region{Quotas: quotas, Name: "Europe"})
	regionList := &compute.RegionList{Items: regions}
	return regionList, nil
}

type MockQuotaCache struct {
}

func (m *MockQuotaCache) Set(k string, x interface{}, d time.Duration) {}

func (m *MockQuotaCache) Items() map[string]cache.Item {
	var result = map[string]cache.Item{
		"1": {
			Expiration: 0,
			Object: &Result{
				region: "region",
				project: &compute.Project{
					Id:   uint64(23),
					Name: "projectName",
				},
				quotaResult: &common.QuotaResult{
					QuotaCode:    "code",
					QuotaName:    "name",
					LimitValue:   100,
					CurrentValue: 30,
				},
			},
		},
	}
	return result
}

func TestGcpQuota(t *testing.T) {
	uri := "/metrics"
	cred := vault.CloudCredentials{
		vault.GcpServiceAccount: "{\"type\": \"service_account\"}",
		vault.GcpProjectID:      "projectID",
	}
	vaultBackupBucket := config.VaultBackupBucketConfig{
		Bucket: "mock_bucket",
		Prefix: "mock_prefix",
	}
	conf := &config.Config{
		VaultBackupBucket: &vaultBackupBucket,
		Region:            "eu-central-1",
	}
	quotaCollector := NewMetricsCollectorGcpRmQuota(conf, cred, &log.Logger{})
	quotaCollector.client = &MockServiceClient{}
	common.QuotaCache = &MockQuotaCache{}
	quotaCollector.scrape(context.TODO())
	registry := prometheus.NewRegistry()
	registry.MustRegister(quotaCollector)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	})

	assert.HTTPBodyContains(t, handler, "GET", uri, nil, "cpe_quota_current{ProjectID=\"23\",ProjectName=\"projectName\",QuotaCode=\"code\",QuotaName=\"name\",Region=\"region\",Regional=\"false\"} 30\n")
	assert.HTTPBodyContains(t, handler, "GET", uri, nil, "cpe_quota_limit{ProjectID=\"23\",ProjectName=\"projectName\",QuotaCode=\"code\",QuotaName=\"name\",Region=\"region\",Regional=\"false\"} 100\n")
}
