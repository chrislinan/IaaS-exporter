package azure

import (
	"context"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/common"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
	"time"
)

type IAzureClient interface {
	ContainerService(bucketName string, accountName string) (IContainer, error)
}

type ContainerWrapper struct {
	client *AzureClient
}

func (cw *ContainerWrapper) ContainerService(bucketName string, accountName string) (IContainer, error) {
	return cw.client.ContainerService(bucketName, accountName)
}

type IContainer interface {
	ListBlobsFlatSegment(ctx context.Context, marker azblob.Marker, o azblob.ListBlobsSegmentOptions) (*azblob.ListBlobsFlatSegmentResponse, error)
}

type MetricsCollectorAzureVaultBucket struct {
	conf   *config.Config
	client IAzureClient
}

func NewMetricsCollectorAzureVaultBucket(config *config.Config, cred vault.CloudCredentials) *MetricsCollectorAzureVaultBucket {
	m := &MetricsCollectorAzureVaultBucket{}
	storageClient, err := NewAzureClient(cred[vault.AzureClientID], cred[vault.AzureClientSecret], cred[vault.AzureTenantID], cred[vault.AzureSubscriptionID], config.VaultBackupBucket.Bucket)
	if err != nil {
		log.Fatal("Error while getting storage client: ", err)
	}
	c := ContainerWrapper{client: storageClient}
	m.conf = config
	m.client = &c

	common.InitVaultBackupBucketDesc("", "AZURE", []string{"bucket", "prefix"})
	return m
}

func (m *MetricsCollectorAzureVaultBucket) Describe(ch chan<- *prometheus.Desc) {
	ch <- common.ListSuccess
	ch <- common.LastModifiedObjectDate
	ch <- common.LastModifiedObjectSize
	ch <- common.ObjectTotal
	ch <- common.SumSize
	ch <- common.BiggestSize
}

func (m *MetricsCollectorAzureVaultBucket) Collect(ch chan<- prometheus.Metric) {
	var lastModified time.Time
	var createdTime time.Time
	var numberOfObjects float64
	var totalSize int64
	var biggestObjectSize int64
	var lastObjectSize int64

	bucketName := m.conf.VaultBackupBucket.Bucket
	accountName := bucketName
	prefix := m.conf.VaultBackupBucket.Prefix

	containerService, err := m.client.ContainerService(bucketName, accountName)
	if err != nil {
		log.Fatalf("could not create container service for bucket %s: %v", bucketName, err)
	}

	options := azblob.ListBlobsSegmentOptions{
		Details: azblob.BlobListingDetails{Snapshots: false, Metadata: true},
		Prefix:  prefix,
	}

	for marker := (azblob.Marker{}); marker.NotDone(); {
		var listBlob *azblob.ListBlobsFlatSegmentResponse

		listBlob, err = containerService.ListBlobsFlatSegment(context.Background(), marker, options)

		if err != nil {
			log.Fatalf("could not list objects in bucket %s: %v", bucketName, err)
			ch <- prometheus.MustNewConstMetric(
				common.ListSuccess, prometheus.GaugeValue, 0, bucketName, prefix,
			)
		}

		marker = listBlob.NextMarker

		for _, blobInfo := range listBlob.Segment.BlobItems {
			numberOfObjects++
			totalSize = totalSize + *blobInfo.Properties.ContentLength
			if blobInfo.Properties.LastModified.After(lastModified) {
				lastModified = blobInfo.Properties.LastModified
				lastObjectSize = *blobInfo.Properties.ContentLength
			}
			if *blobInfo.Properties.ContentLength > biggestObjectSize {
				biggestObjectSize = *blobInfo.Properties.ContentLength
			}
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
