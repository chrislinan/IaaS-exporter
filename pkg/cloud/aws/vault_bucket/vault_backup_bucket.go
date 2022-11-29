package vault_bucket

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConf "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/common"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
	"time"
)

type IS3Client interface {
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

type MetricsCollectorAWSVaultBucket struct {
	conf     *config.Config
	s3Client IS3Client
	log      log.FieldLogger
}

func NewMetricsCollectorAWSVaultBucket(config *config.Config, cred vault.CloudCredentials, logger log.FieldLogger) *MetricsCollectorAWSVaultBucket {
	m := &MetricsCollectorAWSVaultBucket{}
	m.conf = config
	m.log = logger
	//s3 hostname: s3.Region.amazonaws.com
	cfg, err := awsConf.LoadDefaultConfig(context.TODO(), awsConf.WithCredentialsProvider(credentials.StaticCredentialsProvider{Value: aws.Credentials{
		AccessKeyID:     cred[vault.AwsAccessKeyID],
		SecretAccessKey: cred[vault.AwsSecretAccessKey],
	}}), awsConf.WithRegion(config.Region))
	if err != nil {
		m.log.Fatal(err)
	}
	m.s3Client = s3.NewFromConfig(cfg)
	common.InitVaultBackupBucketDesc("", "AWS", []string{"bucket", "prefix"})
	return m
}

// Describe all the metrics we export
func (m *MetricsCollectorAWSVaultBucket) Describe(ch chan<- *prometheus.Desc) {
	ch <- common.ListSuccess
	ch <- common.LastModifiedObjectDate
	ch <- common.LastModifiedObjectSize
	ch <- common.ObjectTotal
	ch <- common.SumSize
	ch <- common.BiggestSize
}

// Collect metrics
func (m *MetricsCollectorAWSVaultBucket) Collect(ch chan<- prometheus.Metric) {
	var lastModified time.Time
	var numberOfObjects float64
	var totalSize int64
	var biggestObjectSize int64
	var lastObjectSize int64

	bucketName := m.conf.VaultBackupBucket.Bucket
	prefix := m.conf.VaultBackupBucket.Prefix

	query := &s3.ListObjectsV2Input{
		Bucket: &bucketName,
		Prefix: &prefix,
	}

	// Continue making requests until we've listed and compared the date of every object
	truncated := true
	for truncated {
		resp, err := m.s3Client.ListObjectsV2(context.TODO(), query)
		if err != nil {
			log.Error(err)
			ch <- prometheus.MustNewConstMetric(
				common.ListSuccess, prometheus.GaugeValue, 0, bucketName, prefix,
			)
			return
		}
		for _, item := range resp.Contents {
			numberOfObjects++
			totalSize = totalSize + item.Size
			if item.LastModified.After(lastModified) {
				lastModified = *item.LastModified
				lastObjectSize = item.Size
			}
			if item.Size > biggestObjectSize {
				biggestObjectSize = item.Size
			}
		}
		query.ContinuationToken = resp.NextContinuationToken
		truncated = resp.IsTruncated
	}

	ch <- prometheus.MustNewConstMetric(
		common.ListSuccess, prometheus.GaugeValue, 1, bucketName, prefix,
	)
	ch <- prometheus.MustNewConstMetric(
		common.LastModifiedObjectDate, prometheus.GaugeValue, float64(lastModified.UnixNano()/1e9), bucketName, prefix,
	)
	ch <- prometheus.MustNewConstMetric(
		common.LastModifiedObjectSize, prometheus.GaugeValue, float64(lastObjectSize), bucketName, prefix,
	)
	ch <- prometheus.MustNewConstMetric(
		common.ObjectTotal, prometheus.GaugeValue, numberOfObjects, bucketName, prefix,
	)
	ch <- prometheus.MustNewConstMetric(
		common.BiggestSize, prometheus.GaugeValue, float64(biggestObjectSize), bucketName, prefix,
	)
	ch <- prometheus.MustNewConstMetric(
		common.SumSize, prometheus.GaugeValue, float64(totalSize), bucketName, prefix,
	)
}
