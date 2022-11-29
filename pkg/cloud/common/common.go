package common

import (
	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/constant"
	"strings"
	"time"
)

var (
	QuotaCurrent *prometheus.GaugeVec
	QuotaLimit   *prometheus.GaugeVec

	HealthEvent       *prometheus.CounterVec
	AffectedEntity    *prometheus.CounterVec
	HealthOpenedTotal *prometheus.CounterVec
	HealthClosedTotal *prometheus.CounterVec

	ListSuccess            *prometheus.Desc
	LastModifiedObjectDate *prometheus.Desc
	LastModifiedObjectSize *prometheus.Desc
	ObjectTotal            *prometheus.Desc
	SumSize                *prometheus.Desc
	BiggestSize            *prometheus.Desc
	QuotaCache             ICache
)

type ICache interface {
	Set(k string, x interface{}, d time.Duration)
	Items() map[string]cache.Item
}

type QuotaResult struct {
	QuotaName    string
	QuotaCode    string
	LimitValue   float64
	CurrentValue float64
	Unit         string
}

func InitQuotaGaugeVec(cloudProvider string, labels []string) {
	QuotaCurrent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: constant.QuotaCurrent,
			Help: strings.Join([]string{cloudProvider, constant.HelpQuotaCurrent}, " "),
		}, labels,
	)

	QuotaLimit = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: constant.QuotaLimit,
			Help: strings.Join([]string{cloudProvider, constant.HelpQuotaLimit}, " "),
		}, labels,
	)
}

func InitHealthCounterVec(cloudProvider string, eventLabels, entityLabels, openedTotalLabels, closedTotalLabels []string) {
	HealthEvent = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: constant.HealthEvent,
			Help: strings.Join([]string{cloudProvider, constant.HelpHealthEvent}, " "),
		}, eventLabels,
	)

	AffectedEntity = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: constant.HealthAffected,
			Help: strings.Join([]string{cloudProvider, constant.HelpHealthAffected}, " "),
		}, entityLabels,
	)

	HealthOpenedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: constant.HealthEventOpenTotal,
			Help: strings.Join([]string{cloudProvider, constant.HelpHealthEventOpenedTotal}, " "),
		}, openedTotalLabels,
	)

	HealthClosedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: constant.HealthEventCloseTotal,
			Help: strings.Join([]string{cloudProvider, constant.HelpHealthEventClosedTotal}, " "),
		}, closedTotalLabels,
	)
}

func InitVaultBackupBucketDesc(namespace, cloudProvider string, labels []string) {
	ListSuccess = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", constant.VaultListSuccess),
		cloudProvider+constant.HelpVaultBackupBucketListSuccess,
		labels, nil,
	)
	LastModifiedObjectDate = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", constant.VaultLastModifyDate),
		cloudProvider+constant.HelpVaultBackupBucketLastModifiedObjectDate,
		labels, nil,
	)
	LastModifiedObjectSize = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", constant.VaultLastModifySize),
		cloudProvider+constant.HelpVaultBackupBucketLastModifiedObjectSize,
		labels, nil,
	)
	ObjectTotal = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", constant.VaultObjectCount),
		cloudProvider+constant.HelpVaultBackupBucketObjectTotal,
		labels, nil,
	)
	SumSize = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", constant.VaultObjectSizeTotal),
		cloudProvider+constant.HelpVaultBackupBucketSumSize,
		labels, nil,
	)
	BiggestSize = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", constant.VaultMaxSize),
		cloudProvider+constant.HelpVaultBackupBucketBiggestSize,
		labels, nil,
	)
}
