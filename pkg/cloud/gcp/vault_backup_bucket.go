package gcp

import (
	"cloud.google.com/go/storage"
	"context"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/common"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"time"
)

type MetricsCollectorGcpVaultBucket struct {
	conf   *config.Config
	client IClient
}

type IClient interface {
	GetObjects(bucketName string, prefix string, ctx context.Context) IIter
}

type ClientWrapper struct {
	*storage.Client
}

func (cw *ClientWrapper) GetObjects(bucketName string, prefix string, ctx context.Context) IIter {
	bucket := cw.Client.Bucket(bucketName)
	query := &storage.Query{Prefix: prefix}
	return bucket.Objects(ctx, query)
}

type IIter interface {
	Next() (*storage.ObjectAttrs, error)
}

func NewMetricsCollectorGcpVaultBucket(config *config.Config, cred vault.CloudCredentials) *MetricsCollectorGcpVaultBucket {
	m := &MetricsCollectorGcpVaultBucket{}
	ctx := context.Background()
	credentials, err := google.CredentialsFromJSON(context.Background(), []byte(cred[vault.GcpServiceAccount]))
	if err != nil {
		log.Fatal(err)
	}
	storageClient, err := storage.NewClient(ctx, option.WithCredentials(credentials))
	client := ClientWrapper{storageClient}
	m.client = &client
	m.conf = config
	common.InitVaultBackupBucketDesc("", "GCP", []string{"bucket", "prefix"})
	return m
}

func (m *MetricsCollectorGcpVaultBucket) Describe(ch chan<- *prometheus.Desc) {
	ch <- common.ListSuccess
	ch <- common.LastModifiedObjectDate
	ch <- common.LastModifiedObjectSize
	ch <- common.ObjectTotal
	ch <- common.SumSize
	ch <- common.BiggestSize
}

func (m *MetricsCollectorGcpVaultBucket) Collect(ch chan<- prometheus.Metric) {
	var createdTime time.Time
	var numberOfObjects float64
	var totalSize int64
	var biggestObjectSize int64
	var lastObjectSize int64

	ctx := context.Background()
	bucketName := m.conf.VaultBackupBucket.Bucket
	prefix := m.conf.VaultBackupBucket.Prefix
	it := m.client.GetObjects(bucketName, prefix, ctx)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Errorln(err)
			ch <- prometheus.MustNewConstMetric(
				common.ListSuccess, prometheus.GaugeValue, 0, bucketName, prefix,
			)
			return
		}
		numberOfObjects++
		totalSize = totalSize + attrs.Size
		if attrs.Created.After(createdTime) {
			createdTime = attrs.Created
			lastObjectSize = attrs.Size
		}
		if attrs.Size > biggestObjectSize {
			biggestObjectSize = attrs.Size
		}
	}

	ch <- prometheus.MustNewConstMetric(
		common.ListSuccess, prometheus.GaugeValue, 1, bucketName, prefix,
	)
	ch <- prometheus.MustNewConstMetric(
		common.LastModifiedObjectDate, prometheus.GaugeValue, float64(createdTime.UnixNano()/1e9), bucketName, prefix,
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
