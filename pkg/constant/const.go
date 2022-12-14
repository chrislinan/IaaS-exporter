package constant

const (
	ProviderAws                                 = "aws"
	ProviderAzure                               = "azure"
	ProviderAliCloud                            = "alicloud"
	ProviderGcp                                 = "gcp"
	ErrUnknownProvider                          = "unknown cloud provider: %s"
	QuotaCurrent                                = "cpe_quota_current"
	QuotaLimit                                  = "cpe_quota_limit"
	VaultListSuccess                            = "cpe_vault_object_list_success"
	VaultMaxSize                                = "cpe_vault_object_max_size_bytes"
	VaultLastModifyDate                         = "cpe_vault_object_last_modified_date"
	VaultLastModifySize                         = "cpe_vault_object_last_modified_size_bytes"
	VaultObjectCount                            = "cpe_vault_object_count"
	VaultObjectSizeTotal                        = "cpe_vault_object_size_bytes_total"
	HealthEvent                                 = "cpe_health_events"
	HealthAffected                              = "cpe_health_events_affected"
	HealthEventOpenTotal                        = "cpe_health_events_opened_total"
	HealthEventCloseTotal                       = "cpe_health_events_closed_total"
	HelpHealthAffected                          = "Resource health affected information"
	HelpHealthEvent                             = "Resource health event information"
	HelpHealthEventOpenedTotal                  = "Resource health opened total"
	HelpHealthEventClosedTotal                  = "Resource health closed total"
	LabelServiceName                            = "ServiceName"
	LabelServiceCode                            = "ServiceCode"
	LabelQuotaCode                              = "QuotaCode"
	LabelQuotaName                              = "QuotaName"
	LabelAccountID                              = "AccountID"
	LabelAccountAlias                           = "AccountAlias"
	LabelRegion                                 = "Region"
	LabelUnit                                   = "Unit"
	LabelRegional                               = "Regional"
	LabelProjectID                              = "ProjectID"
	LabelProjectName                            = "ProjectName"
	LabelSubscriptionID                         = "SubscriptionID"
	LabelSubscriptionName                       = "SubscriptionName"
	LabelProductCode                            = "ProductCode"
	LabelQuotaDescription                       = "QuotaDescription"
	HelpQuotaCurrent                            = "Current usage value of quota"
	HelpQuotaLimit                              = "Limit value of quota"
	HelpVaultBackupBucketListSuccess            = "If the ListObjects operation was a success"
	HelpVaultBackupBucketLastModifiedObjectDate = "The last modified date of the object that was modified most recently"
	HelpVaultBackupBucketLastModifiedObjectSize = "The size of the object that was modified most recently"
	HelpVaultBackupBucketObjectTotal            = "The total number of objects for the bucket/prefix combination"
	HelpVaultBackupBucketSumSize                = "The total size of all objects summed"
	HelpVaultBackupBucketBiggestSize            = "The size of the biggest object"
	VaultMonitorPath                            = "/bucket"
	MetricsMonitorPath                          = "/monitor"
	GCPQuotaScope                               = "https://www.googleapis.com/auth/compute.readonly"
)
