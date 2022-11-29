package alicloud

import (
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/constant"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
	"net/http"
	"testing"
	"time"
)

type MockClient struct {
}

func (m *MockClient) Bucket(name string) (IBucket, error) {
	var mockbucket MockBucket
	return &mockbucket, nil
}

type MockBucket struct {
}

func (m *MockBucket) ListObjects(options ...oss.Option) (oss.ListObjectsResult, error) {
	return oss.ListObjectsResult{
		Objects: []oss.ObjectProperties{
			{
				Key:          "one",
				LastModified: time.Date(2019, time.June, 13, 21, 0, 0, 0, time.UTC),
				Size:         100,
			},
			{
				Key:          "two",
				LastModified: time.Date(2020, time.June, 13, 21, 0, 0, 0, time.UTC),
				Size:         200,
			},
		},
		IsTruncated: false,
		MaxKeys:     1000,
		Prefix:      "prefix",
	}, nil
}

func TestAlicloudVaultBucket(t *testing.T) {
	uri := constant.VaultMonitorPath
	cred := vault.CloudCredentials{
		vault.AliCloudAccessKeyID:     "accessKeyID",
		vault.AliCloudSecretAccessKey: "secretAccessKey",
	}

	vaultBackupBucket := config.VaultBackupBucketConfig{
		Bucket: "mock_bucket",
		Prefix: "mock_prefix",
	}
	conf := &config.Config{
		VaultBackupBucket: &vaultBackupBucket,
		Region:            "eu-central-1",
		AliCloud: &config.AliCloudConfig{
			Endpoint: "oss-cn-hangzhou.aliyuncs.com",
		},
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vaultBucketCollector := NewMetricsCollectorAliVaultBucket(conf, cred)
		vaultBucketCollector.client = &MockClient{}
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
