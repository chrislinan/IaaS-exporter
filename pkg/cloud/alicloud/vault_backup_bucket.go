package alicloud

import (
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/common"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
	"time"
)

type MetricsCollectorAliVaultBucket struct {
	conf   *config.Config
	client IClient
}

func NewMetricsCollectorAliVaultBucket(config *config.Config, cred vault.CloudCredentials) *MetricsCollectorAliVaultBucket {
	m := &MetricsCollectorAliVaultBucket{}
	client, err := oss.New(config.AliCloud.Endpoint, cred[vault.AliCloudAccessKeyID], cred[vault.AliCloudSecretAccessKey])
	c := ClientWrapper{client}
	if err != nil {
		log.Errorln("Error creating client ", err)
	}
	m.client = c
	m.conf = config
	common.InitVaultBackupBucketDesc("", "ALICLOUD", []string{"bucket", "prefix"})
	return m
}

type IClient interface {
	Bucket(name string) (IBucket, error)
}

type ClientWrapper struct {
	*oss.Client
}

func (cw ClientWrapper) Bucket(name string) (IBucket, error) {
	return cw.Client.Bucket(name)
}

type IBucket interface {
	ListObjects(options ...oss.Option) (oss.ListObjectsResult, error)
}

func (m *MetricsCollectorAliVaultBucket) Describe(ch chan<- *prometheus.Desc) {
	ch <- common.ListSuccess
	ch <- common.LastModifiedObjectDate
	ch <- common.LastModifiedObjectSize
	ch <- common.ObjectTotal
	ch <- common.SumSize
	ch <- common.BiggestSize
}
func (m *MetricsCollectorAliVaultBucket) Collect(ch chan<- prometheus.Metric) {
	var lastModified time.Time
	var numberOfObjects float64
	var totalSize int64
	var biggestObjectSize int64
	var lastObjectSize int64
	bucketName := m.conf.VaultBackupBucket.Bucket
	prefix := m.conf.VaultBackupBucket.Prefix
	bucket, err := m.client.Bucket(bucketName)
	if err != nil {
		log.Errorln(err)
		ch <- prometheus.MustNewConstMetric(
			common.ListSuccess, prometheus.GaugeValue, 0, bucketName, prefix,
		)
		return
	}

	//list all objects in bucket
	marker := ""
	for {
		lsRes, err := bucket.ListObjects(oss.Prefix(prefix), oss.Marker(marker))
		if err != nil {
			log.Errorln(err)
			ch <- prometheus.MustNewConstMetric(
				common.ListSuccess, prometheus.GaugeValue, 0, bucketName, prefix,
			)
			return
		}
		for _, item := range lsRes.Objects {
			numberOfObjects++
			totalSize = totalSize + item.Size
			if item.LastModified.After(lastModified) {
				lastModified = item.LastModified
				lastObjectSize = item.Size
			}
			if item.Size > biggestObjectSize {
				biggestObjectSize = item.Size
			}
		}
		if lsRes.IsTruncated {
			marker = lsRes.NextMarker
		} else {
			break
		}
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
