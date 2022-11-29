package azure

import (
	"context"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/common"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/constant"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
	"net/http"
	"testing"
	"time"
)

type MockAzureClient struct {
}

func (m *MockAzureClient) ContainerService(bucketName string, accountName string) (IContainer, error) {
	var container MockContainer
	return &container, nil
}

type MockContainer struct {
}

func (m *MockContainer) ListBlobsFlatSegment(ctx context.Context, marker azblob.Marker, o azblob.ListBlobsSegmentOptions) (*azblob.ListBlobsFlatSegmentResponse, error) {
	return &azblob.ListBlobsFlatSegmentResponse{
		Segment: azblob.BlobFlatListSegment{
			BlobItems: []azblob.BlobItemInternal{
				{
					Properties: azblob.BlobPropertiesInternal{
						LastModified:  time.Date(2019, time.June, 13, 21, 0, 0, 0, time.UTC),
						ContentLength: to.Int64Ptr(100),
					},
				},
				{
					Properties: azblob.BlobPropertiesInternal{
						LastModified:  time.Date(2020, time.June, 13, 21, 0, 0, 0, time.UTC),
						ContentLength: to.Int64Ptr(200),
					},
				},
			},
		},
		NextMarker: azblob.Marker{
			Val: to.StringPtr(""),
		},
	}, nil
}

func NewMockMetricsCollectorAzureVaultBucket(config *config.Config, cred vault.CloudCredentials) *MetricsCollectorAzureVaultBucket {
	m := &MetricsCollectorAzureVaultBucket{}
	m.conf = config
	m.client = &MockAzureClient{}
	common.InitVaultBackupBucketDesc("", "AZURE", []string{"bucket", "prefix"})
	return m
}

func TestAzureVaultBucket(t *testing.T) {
	uri := constant.VaultMonitorPath
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
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vaultBucketCollector := NewMockMetricsCollectorAzureVaultBucket(conf, cred)
		registry := prometheus.NewRegistry()
		registry.MustRegister(vaultBucketCollector)
		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	})

	assert.HTTPBodyContains(t, handler, "GET", uri, nil, "cpe_vault_object_list_success{bucket=\"mock_bucket\",prefix=\"mock_prefix\"} 1")
	assert.HTTPBodyContains(t, handler, "GET", uri, nil, "cpe_vault_object_last_modified_size_bytes{bucket=\"mock_bucket\",prefix=\"mock_prefix\"} 200")
	assert.HTTPBodyContains(t, handler, "GET", uri, nil, "cpe_vault_object_count{bucket=\"mock_bucket\",prefix=\"mock_prefix\"} 2")
	assert.HTTPBodyContains(t, handler, "GET", uri, nil, "cpe_vault_object_max_size_bytes{bucket=\"mock_bucket\",prefix=\"mock_prefix\"} 200")
	assert.HTTPBodyContains(t, handler, "GET", uri, nil, "cpe_vault_object_size_bytes_total{bucket=\"mock_bucket\",prefix=\"mock_prefix\"} 300")
}
