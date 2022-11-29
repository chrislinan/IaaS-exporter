package factory

import (
	"context"
	log "github.com/sirupsen/logrus"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/alicloud"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/aws"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/azure"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/gcp"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/constant"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
)

type Exporter interface {
	StartExporter(ctx context.Context, config *config.Config, credential vault.CloudCredentials, logger log.FieldLogger)
	Scrape(ctx context.Context)
}

type ExporterFactory struct {
}

func (f *ExporterFactory) NewExporter(provider string) Exporter {
	switch provider {
	case constant.ProviderAws:
		return &aws.AwsExporter{}
	case constant.ProviderAzure:
		return &azure.AzureExporter{}
	case constant.ProviderGcp:
		return &gcp.GcpExporter{}
	case constant.ProviderAliCloud:
		return &alicloud.AliExporter{}
	default:
		return nil
	}
}
