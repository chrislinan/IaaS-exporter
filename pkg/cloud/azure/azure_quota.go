package azure

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/network/mgmt/network"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/storage/mgmt/storage"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/common"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/constant"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
	"sync"
)

type StorageClient interface {
	ListByLocation(ctx context.Context, location string) (result storage.UsageListResult, err error)
	SetAuth(auth autorest.Authorizer)
}

type NetworkClient interface {
	ListComplete(ctx context.Context, location string) (result network.UsagesListResultIterator, err error)
	SetAuth(auth autorest.Authorizer)
}

type ComputeClient interface {
	ListComplete(ctx context.Context, location string) (result compute.ListUsagesResultIterator, err error)
	SetAuth(auth autorest.Authorizer)
}

// To add authorizer into UsageClient, we have to encapsulate storage.UsagesClient, network.UsagesClient, and compute.UsageClient

var (
	subscriptionID   string
	subscriptionName string
)

type StorageClientWrapper struct {
	client storage.UsagesClient
}

func (cw *StorageClientWrapper) ListByLocation(ctx context.Context, location string) (result storage.UsageListResult, err error) {
	return cw.client.ListByLocation(ctx, location)
}

func (cw *StorageClientWrapper) SetAuth(auth autorest.Authorizer) {
	cw.client.Authorizer = auth
}

func NewStorageClientWrapper(id string) *StorageClientWrapper {
	cw := &StorageClientWrapper{client: storage.NewUsagesClient(id)}
	return cw
}

type NetworkClientWrapper struct {
	client network.UsagesClient
}

func (cw *NetworkClientWrapper) ListComplete(ctx context.Context, location string) (result network.UsagesListResultIterator, err error) {
	return cw.client.ListComplete(ctx, location)
}

func (cw *NetworkClientWrapper) SetAuth(auth autorest.Authorizer) {
	cw.client.Authorizer = auth
}

func NewNetworkClientWrapper(id string) *NetworkClientWrapper {
	cw := &NetworkClientWrapper{client: network.NewUsagesClient(id)}
	return cw
}

type ComputeClientWrapper struct {
	client compute.UsageClient
}

func (cw *ComputeClientWrapper) ListComplete(ctx context.Context, location string) (result compute.ListUsagesResultIterator, err error) {
	return cw.client.ListComplete(ctx, location)
}

func (cw *ComputeClientWrapper) SetAuth(auth autorest.Authorizer) {
	cw.client.Authorizer = auth
}

func NewComputeClientWrapper(id string) *ComputeClientWrapper {
	cw := &ComputeClientWrapper{client: compute.NewUsageClient(id)}
	return cw
}

type SubscriptionInfo interface {
	GetSubscriptionInfo(logger log.FieldLogger) (subscriptionID, subscriptionName string)
}

type SubscriptionInfoWrapper struct {
	config *config.Config
	cred   vault.CloudCredentials
}

func (s *SubscriptionInfoWrapper) GetSubscriptionInfo(logger log.FieldLogger) (subscriptionID, subscriptionName string) {
	clientCredentialConfig := auth.NewClientCredentialsConfig(s.cred[vault.AzureClientID], s.cred[vault.AzureClientSecret], s.cred[vault.AzureTenantID])
	authorizer, err := clientCredentialConfig.Authorizer()
	if err != nil {
		logger.Errorf("Error while getting authorizer: ", err)
	}
	subscriptionClient := subscriptions.NewClient()
	subscriptionClient.Authorizer = authorizer
	subscriptionInfo, err := subscriptionClient.Get(context.Background(), s.config.Azure.SubscriptionID)
	if err != nil {
		logger.Errorf("Error while getting subscription Info: ", err)
		return "", ""
	}
	subscriptionID = s.config.Azure.SubscriptionID
	subscriptionName = *subscriptionInfo.DisplayName
	return subscriptionID, subscriptionName
}

type MetricsCollectorAzureRmQuota struct {
	conf               *config.Config
	log                log.FieldLogger
	storageUsageClient StorageClient
	networkUsageClient NetworkClient
	computeUsageClient ComputeClient
	subscriptionInfo   SubscriptionInfo
}

func NewMetricsCollectorAzureRmQuota(config *config.Config, cred vault.CloudCredentials, logger log.FieldLogger) *MetricsCollectorAzureRmQuota {
	clientCredentialConfig := auth.NewClientCredentialsConfig(cred[vault.AzureClientID], cred[vault.AzureClientSecret], cred[vault.AzureTenantID])
	authorizer, err := clientCredentialConfig.Authorizer()
	if err != nil {
		logger.Errorf("Error while getting authorizer: ", err)
	}
	m := &MetricsCollectorAzureRmQuota{}
	m.conf = config
	m.log = logger
	m.log.Infof("Initialize Azure storage client")
	m.storageUsageClient = NewStorageClientWrapper(m.conf.Azure.SubscriptionID)
	m.storageUsageClient.SetAuth(authorizer)

	m.log.Infof("Initialize Azure Network client")
	m.networkUsageClient = NewNetworkClientWrapper(m.conf.Azure.SubscriptionID)
	m.networkUsageClient.SetAuth(authorizer)

	m.log.Infof("Initialize Azure compute client")
	m.computeUsageClient = NewComputeClientWrapper(m.conf.Azure.SubscriptionID)
	m.computeUsageClient.SetAuth(authorizer)

	m.subscriptionInfo = &SubscriptionInfoWrapper{config, cred}

	common.InitQuotaGaugeVec("Azure", []string{constant.LabelRegion, constant.LabelQuotaCode, constant.LabelQuotaName, constant.LabelSubscriptionID, constant.LabelSubscriptionName, constant.LabelUnit})
	return m
}

func (m *MetricsCollectorAzureRmQuota) Describe(ch chan<- *prometheus.Desc) {
	common.QuotaLimit.Describe(ch)
	common.QuotaCurrent.Describe(ch)
}

func (m *MetricsCollectorAzureRmQuota) collectCompute(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	m.log.Infof("Start collect Azure compute metrics")
	var currentValue, limitValue float64
	for usage, err := m.computeUsageClient.ListComplete(ctx, m.conf.Region); usage.NotDone(); err = usage.NextWithContext(ctx) {
		if err != nil {
			m.log.Errorf("Error while traversing compute resource list: ", err)
			return
		}
		i := usage.Value()
		currentValue = float64(to.Int32(i.CurrentValue))
		limitValue = float64(to.Int64(i.Limit))
		if limitValue >= 2147483647 { // unlimited resource
			continue
		}
		code := to.String(i.Name.Value)
		name := to.String(i.Name.LocalizedValue)
		if currentValue > 0 {
			result := &common.QuotaResult{QuotaCode: code, QuotaName: name, CurrentValue: currentValue, LimitValue: limitValue, Unit: to.String(i.Unit)}
			common.QuotaCache.Set(code, result, cache.DefaultExpiration)
		}
	}
	m.log.Infof("End collect Azure compute metrics")
}

func (m *MetricsCollectorAzureRmQuota) collectStorage(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	m.log.Infof("Start collect Azure storage metrics")
	var currentValue, limitValue float64
	storageUsageList, err := m.storageUsageClient.ListByLocation(ctx, m.conf.Region)
	if err != nil {
		m.log.Errorf("Error while traversing storage resource list: ", err)
		return
	}
	for _, i := range *storageUsageList.Value {
		currentValue = float64(to.Int32(i.CurrentValue))
		limitValue = float64(to.Int32(i.Limit))
		if limitValue >= 2147483647 { // unlimited resource
			continue
		}
		code := to.String(i.Name.Value)
		name := to.String(i.Name.LocalizedValue)
		if currentValue > 0 {
			result := &common.QuotaResult{QuotaCode: code, QuotaName: name, CurrentValue: currentValue, LimitValue: limitValue, Unit: string(i.Unit)}
			common.QuotaCache.Set(code, result, cache.DefaultExpiration)
		}
	}
	m.log.Infof("End collect Azure storage metrics")
}

func (m *MetricsCollectorAzureRmQuota) collectNetwork(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	m.log.Infof("Start collect Azure network metrics")
	var currentValue, limitValue float64
	for usage, err := m.networkUsageClient.ListComplete(ctx, m.conf.Region); usage.NotDone(); err = usage.NextWithContext(ctx) {
		if err != nil {
			m.log.Errorf("Error while traversing network resource list: ", err)
			return
		}
		i := usage.Value()
		currentValue = float64(to.Int64(i.CurrentValue))
		limitValue = float64(to.Int64(i.Limit))
		if limitValue >= 2147483647 { // unlimited resource
			continue
		}
		code := to.String(i.Name.Value)
		name := to.String(i.Name.LocalizedValue)
		if currentValue > 0 {
			result := &common.QuotaResult{QuotaCode: code, QuotaName: name, CurrentValue: currentValue, LimitValue: limitValue, Unit: to.String(i.Unit)}
			common.QuotaCache.Set(code, result, cache.DefaultExpiration)
		}
	}
	m.log.Infof("End collect Azure network metrics")
}

func (m *MetricsCollectorAzureRmQuota) scrape(ctx context.Context) {
	m.log.Infof("Start collect Azure metrics")
	var waitGroup sync.WaitGroup
	waitGroup.Add(3)
	go m.collectNetwork(ctx, &waitGroup)
	go m.collectStorage(ctx, &waitGroup)
	go m.collectCompute(ctx, &waitGroup)
	waitGroup.Wait()
	m.log.Infof("End collect Azure metrics")
}

func (m *MetricsCollectorAzureRmQuota) Collect(ch chan<- prometheus.Metric) {
	m.log.Infof("Start retrieve data from cache")
	subscriptionID, subscriptionName = m.subscriptionInfo.GetSubscriptionInfo(m.log)
	for _, item := range common.QuotaCache.Items() {
		result := item.Object.(*common.QuotaResult)
		m.log.WithFields(log.Fields{"region": m.conf.Region, "quotaCode": result.QuotaCode, "subscriptionID": subscriptionID, "subscriptionName": subscriptionName, "current": result.CurrentValue, "limit": result.LimitValue}).Infof("retrieve data from cache")
		common.QuotaCurrent.WithLabelValues(m.conf.Region, result.QuotaCode, result.QuotaName, subscriptionID, subscriptionName, result.Unit).Set(result.CurrentValue)
		common.QuotaLimit.WithLabelValues(m.conf.Region, result.QuotaCode, result.QuotaName, subscriptionID, subscriptionName, result.Unit).Set(result.LimitValue)
	}
	common.QuotaCurrent.Collect(ch)
	common.QuotaLimit.Collect(ch)
}
