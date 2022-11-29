package azure

import (
	"context"
	"fmt"
	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
	"net/http"
	"net/url"
	"time"
)

type AzureExporter struct {
	quotaCollector  *MetricsCollectorAzureRmQuota
	healthCollector *MetricsCollectorAzureRmHealth
}

func (e *AzureExporter) StartExporter(ctx context.Context, config *config.Config, credential vault.CloudCredentials, logger log.FieldLogger) {
	e.quotaCollector = NewMetricsCollectorAzureRmQuota(config, credential, logger)
	prometheus.MustRegister(e.quotaCollector)
	//e.healthCollector = NewMetricsCollectorAzureRmHealth(config, credential, logger)
	//prometheus.MustRegister(e.healthCollector)
	//http.HandleFunc(constant.VaultMonitorPath, func(w http.ResponseWriter, r *http.Request) {
	//	vaultBackupMonitorHandler(w, r, config, credential)
	//})
}

func (e *AzureExporter) Scrape(ctx context.Context) {
	e.quotaCollector.scrape(ctx)
}

func vaultBackupMonitorHandler(w http.ResponseWriter, r *http.Request, config *config.Config, credential vault.CloudCredentials) {
	vaultBucketCollector := NewMetricsCollectorAzureVaultBucket(config, credential)
	registry := prometheus.NewRegistry()
	registry.MustRegister(vaultBucketCollector)
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}

type AzureClient struct {
	Credentials    azblob.Credential
	Context        context.Context
	BucketName     string
	AccountName    string
	TenantId       string
	SubscriptionId string
}

var (
	AzureEnv *azure.Environment
)

func initAzureEnv() error {
	env, err := azure.EnvironmentFromName("AZUREPUBLICCLOUD")
	AzureEnv = &env
	return err
}

func GetAzureOAuthConfig(azureTenantID string) (*adal.OAuthConfig, error) {
	if AzureEnv == nil {
		err := initAzureEnv()
		if err != nil {
			return nil, err
		}
	}
	azureOAuthConfig, err := adal.NewOAuthConfig(AzureEnv.ActiveDirectoryEndpoint, azureTenantID)
	if err != nil {
		return nil, err
	}
	if azureOAuthConfig == nil {
		return nil, fmt.Errorf("unable to configure authentication for Azure tenant %s", azureTenantID)
	}

	return azureOAuthConfig, nil
}

func getAzureServicePrincipalToken(clientId, clientSecret, azureTenantID string) (*adal.ServicePrincipalToken, error) {
	oauthConfig, err := GetAzureOAuthConfig(azureTenantID)
	if err != nil {
		return nil, err
	}

	spt, err := adal.NewServicePrincipalToken(*oauthConfig, clientId, clientSecret, AzureEnv.ResourceIdentifiers.Storage)
	if err != nil {
		return nil, err
	}

	return spt, nil
}

// GetAzureStorageCredentials returns a azblob.Credential object that can be used to authenticate an Azure Blob Storage SDK pipeline
func GetAzureStorageCredentials(clientId, clientSecret, azureTenantID string) (azblob.Credential, error) {
	err := initAzureEnv()
	if err != nil {
		return nil, err
	}
	spt, err := getAzureServicePrincipalToken(clientId, clientSecret, azureTenantID)
	if err != nil {
		return nil, err
	}

	tokenRefresher := func(credential azblob.TokenCredential) time.Duration {
		err := spt.Refresh()
		if err != nil {
			log.Fatal(err)
		}
		token := spt.Token()
		credential.SetToken(token.AccessToken)
		expires := token.Expires().Sub(time.Now().Add(2 * time.Minute))
		return expires
	}

	credential := azblob.NewTokenCredential("", tokenRefresher)
	return credential, nil
}
func NewAzureClient(clientId, clientSecret, tenantId, subscriptionID, bucketName string) (*AzureClient, error) {
	credentials, err := GetAzureStorageCredentials(clientId, clientSecret, tenantId)
	if err != nil {
		return nil, err
	}

	client := &AzureClient{
		Credentials:    credentials,
		Context:        context.Background(),
		BucketName:     bucketName,
		AccountName:    bucketName,
		TenantId:       tenantId,
		SubscriptionId: subscriptionID,
	}
	return client, nil
}

func (client *AzureClient) BlobService(accountName string) (*azblob.ServiceURL, error) {
	if AzureEnv == nil {
		if err := initAzureEnv(); err != nil {
			return nil, err
		}
	}
	azureURL, err := url.Parse(fmt.Sprintf("https://%s.blob.%s", accountName, AzureEnv.StorageEndpointSuffix))
	if err != nil {
		return nil, fmt.Errorf("failed to parse Azure Blob URL for account bucketName %q: %w", accountName, err)
	}

	pipelineOptions := azblob.PipelineOptions{
		Log: pipeline.LogOptions{
			Log: func(level pipeline.LogLevel, message string) {
				log.Printf("azblob: %v", message)
			},
		},
	}
	httpPipeline := azblob.NewPipeline(client.Credentials, pipelineOptions)
	blobService := azblob.NewServiceURL(*azureURL, httpPipeline)
	return &blobService, nil
}

func (client *AzureClient) ContainerService(bucketName, accountName string) (*azblob.ContainerURL, error) {
	blobService, err := client.BlobService(accountName)
	if err != nil {
		return nil, err
	}
	containerURL := blobService.NewContainerURL(bucketName)

	return &containerURL, err
}
