package gcp

import (
	"cloud.google.com/go/storage"
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/constant"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
	"google.golang.org/api/iterator"
	"net/http"
	"testing"
	"time"
)

var times = 0

type MockClient struct {
}

func (m *MockClient) GetObjects(bucketName string, prefix string, ctx context.Context) IIter {
	var iter MockIterator
	return &iter
}

type MockIterator struct {
}

func (m *MockIterator) Next() (*storage.ObjectAttrs, error) {
	switch times {
	case 0:
		{
			times++
			return &storage.ObjectAttrs{
				Name:    "one",
				Created: time.Date(2019, time.June, 13, 21, 0, 0, 0, time.UTC),
				Size:    100,
			}, nil
		}
	case 1:
		{
			times++
			return &storage.ObjectAttrs{
				Name:    "two",
				Created: time.Date(2020, time.June, 13, 21, 0, 0, 0, time.UTC),
				Size:    200,
			}, nil
		}
	default:
		{
			times = 0
			return &storage.ObjectAttrs{}, iterator.Done
		}
	}
}

func TestGcpVaultBucket(t *testing.T) {
	uri := constant.VaultMonitorPath
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
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vaultBucketCollector := NewMetricsCollectorGcpVaultBucket(conf, cred)
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
