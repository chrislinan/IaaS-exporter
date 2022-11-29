package vault_bucket

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/constant"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
	"net/http"
	"testing"
	"time"
)

type MockS3Client struct {
}

func (m MockS3Client) ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	return &s3.ListObjectsV2Output{
		Contents: []types.Object{
			{
				Key:          aws.String("one"),
				LastModified: aws.Time(time.Date(2019, time.June, 13, 21, 0, 0, 0, time.UTC)),
				Size:         int64(100),
			},
			{
				Key:          aws.String("two"),
				LastModified: aws.Time(time.Date(2020, time.June, 13, 21, 0, 0, 0, time.UTC)),
				Size:         int64(200),
			},
		},
		IsTruncated: false,
		KeyCount:    int32(1),
		MaxKeys:     int32(1000),
		Name:        aws.String("mock"),
		Prefix:      aws.String("one"),
	}, nil
}

func TestAWSVaultBucket(t *testing.T) {
	uri := constant.VaultMonitorPath
	cred := vault.CloudCredentials{
		vault.AwsAccessKeyID:     "accessKeyID",
		vault.AwsSecretAccessKey: "secretAccessKey",
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
		vaultBucketCollector := NewMetricsCollectorAWSVaultBucket(conf, cred, &log.Logger{})
		vaultBucketCollector.s3Client = &MockS3Client{}
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
