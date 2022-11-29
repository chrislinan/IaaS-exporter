package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	vaultApi "github.com/hashicorp/vault/api"
	log "github.com/sirupsen/logrus"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/constant"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/errorutil"
	"io/ioutil"
	"time"
)

type CloudCredentials map[int]string

type VaultClient struct {
	client *vaultApi.Client
}

const (
	AwsAccessKeyID int = iota
	AwsSecretAccessKey
	AzureClientID
	AzureClientSecret
	AzureSubscriptionID
	AzureTenantID
	AliCloudAccessKeyID
	AliCloudSecretAccessKey
	GcpServiceAccount
	GcpProjectID
)

const (
	VaultMaxAttempts             = 3
	VaultRetrySleepDuration      = 5 * time.Second
	VaultRetryLoginSleepDuration = time.Minute
	ReadWriteCredentialsRole     = "iaas-monitor"
)

var (
	ErrVaultReadSecret     = errors.New("could not read Vault secret")
	ErrVaultSecretNotFound = errors.New("vault secret not found")
)

func NewVaultClient(conf *config.Config) (*VaultClient, error) {
	client, err := vaultApi.NewClient(vaultApi.DefaultConfig())
	if err != nil {
		return nil, err
	}

	if err = client.SetAddress(conf.Vault.VaultAddr); err != nil {
		return nil, err
	}

	if conf.Vault.VaultTokenFromEnv {
		vaultClient := &VaultClient{
			client: client,
		}
		return vaultClient, nil
	}

	if conf.Vault.VaultRole != ReadWriteCredentialsRole {
		return nil, fmt.Errorf("invalid Vault role: %s", conf.Vault.VaultRole)
	}

	clientToken, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return nil, err
	}

	if string(clientToken) == "" {
		return nil, errors.New("vault Client token is empty")
	}

	data := map[string]interface{}{
		"jwt": string(clientToken),
		// Roles are defined in
		// https://github.wdf.sap.corp/hanadatalake/landscape/blob/develop/scripts/hc/drivers/VaultClient.py.
		"role": conf.Vault.VaultRole,
	}

	var secret *vaultApi.Secret

	f := func() error {
		var e error
		if secret, e = client.Logical().Write(conf.Vault.VaultLoginPath, data); e != nil {
			log.Printf("Could not log in to Vault: %v", e)
			return e
		}

		return nil
	}
	if err := errorutil.RetryOnError(f, VaultMaxAttempts, VaultRetryLoginSleepDuration); err != nil {
		return nil, fmt.Errorf("could not log in to Vault: %w", err)
	}

	client.SetToken(secret.Auth.ClientToken)

	vaultClient := &VaultClient{
		client: client,
	}

	return vaultClient, nil
}

func (v *VaultClient) CredentialsFromPath(path string, provider string) (CloudCredentials, error) {
	switch provider {
	case constant.ProviderAws:
		accessKeyID, err := v.readSecretV1(path, "AWS_ACCESS_KEY_ID")
		if err != nil {
			return nil, err
		}

		secretAccessKey, err := v.readSecretV1(path, "AWS_SECRET_ACCESS_KEY")
		if err != nil {
			return nil, err
		}

		return CloudCredentials{
			AwsAccessKeyID:     accessKeyID,
			AwsSecretAccessKey: secretAccessKey,
		}, nil
	case constant.ProviderAzure:
		clientID, err := v.readSecretV1(path, "client_id")
		if err != nil {
			return nil, err
		}

		clientSecret, err := v.readSecretV1(path, "client_secret")
		if err != nil {
			return nil, err
		}

		subscriptionID, err := v.readSecretV1(path, "subscription_id")
		if err != nil {
			return nil, err
		}

		tenantID, err := v.readSecretV1(path, "tenant_id")
		if err != nil {
			return nil, err
		}

		return CloudCredentials{
			AzureClientID:       clientID,
			AzureClientSecret:   clientSecret,
			AzureSubscriptionID: subscriptionID,
			AzureTenantID:       tenantID,
		}, nil
	case constant.ProviderAliCloud:
		accessKeyID, err := v.readSecretV1(path, "ALICLOUD_ACCESS_KEY_ID")
		if err != nil {
			return nil, err
		}

		secretAccessKey, err := v.readSecretV1(path, "ALICLOUD_SECRET_ACCESS_KEY")
		if err != nil {
			return nil, err
		}

		return CloudCredentials{
			AliCloudAccessKeyID:     accessKeyID,
			AliCloudSecretAccessKey: secretAccessKey,
		}, nil
	case constant.ProviderGcp:
		gcpServiceAccount, err := v.readSecretV1(path, "gcp_service_account")
		if err != nil {
			return nil, err
		}

		projectID, err := ReadProjectID(gcpServiceAccount)
		if err != nil {
			return nil, err
		}

		return CloudCredentials{
			GcpServiceAccount: gcpServiceAccount,
			GcpProjectID:      projectID,
		}, nil
	default:
		return nil, fmt.Errorf(constant.ErrUnknownProvider, provider)
	}
}

// readSecretV1 reads a version 1 kv secret.
func (v *VaultClient) readSecretV1(path, key string) (string, error) {
	var secret *vaultApi.Secret

	f := func() error {
		var e error
		if secret, e = v.client.Logical().Read(path); e != nil {
			log.Printf("Could not read Vault secret %q: %v", path, e)
			return e
		}

		return nil
	}
	if err := errorutil.RetryOnError(f, VaultMaxAttempts, VaultRetrySleepDuration); err != nil {
		return "", fmt.Errorf("%s: %w", path, ErrVaultReadSecret)
	}

	if secret == nil {
		return "", fmt.Errorf("%s: %w", path, ErrVaultSecretNotFound)
	}

	if key == "gcp_service_account" {
		serviceAccount, err := json.Marshal(secret.Data)
		if err != nil {
			return "", fmt.Errorf("%s @ %s: %w", key, path, ErrVaultSecretNotFound)
		}
		return string(serviceAccount), nil
	}

	data, ok := secret.Data[key]
	if !ok {
		return "", fmt.Errorf("%s @ %s: %w", key, path, ErrVaultSecretNotFound)
	}

	if data == nil {
		return "", fmt.Errorf("%s @ %s: %w", key, path, ErrVaultSecretNotFound)
	}

	return data.(string), nil
}

func ReadProjectID(serviceAccount string) (string, error) {
	var tempMap map[string]interface{}

	err := json.Unmarshal([]byte(serviceAccount), &tempMap)
	if err != nil {
		return "", err
	}
	return tempMap["project_id"].(string), nil
}
