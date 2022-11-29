package config

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
)

type VaultBackupBucketConfig struct {
	Bucket string `yaml:"bucket"`
	Prefix string `yaml:"prefix"`
}

type AwsConfig struct {
	HealthEventStatusCodes    []string              `yaml:"healthEventStatusCodes,flow"`
	HealthEventTypeCategories []string              `yaml:"healthEventTypeCategories,flow"`
	CloudWatchMetricsConf     CloudWatchMetricsConf `yaml:"cloudwatchMetricsConf"`
}

type GcpConfig struct {
}

type AliCloudConfig struct {
	Endpoint string `yaml:"endpoint"`
}

type AzureConfig struct {
	SubscriptionID string `yaml:"subscriptionID"`
}

type VaultConfig struct {
	VaultTokenFromEnv bool   `yaml:"vaultTokenFromEnv"`
	VaultAddr         string `yaml:"vaultAddr"`
	VaultLoginPath    string `yaml:"vaultLoginPath"`
	VaultRole         string `yaml:"role"`
}

type Config struct {
	Provider                         string                   `yaml:"provider"`
	Project                          string                   `yaml:"project"`
	Region                           string                   `yaml:"region"`
	ScrapingDuration                 int32                    `yaml:"scrapingDuration"`
	CacheExpiration                  int32                    `yaml:"cacheExpiration"`
	CacheCleanupInterval             int32                    `yaml:"cacheCleanupInterval"`
	CloudProviderAccountVaultSubpath string                   `yaml:"cloudProviderAccountVaultSubpath"`
	Aws                              *AwsConfig               `yaml:"AwsConfig"`
	Gcp                              *GcpConfig               `yaml:"GcpConfig"`
	Azure                            *AzureConfig             `yaml:"AzureConfig"`
	AliCloud                         *AliCloudConfig          `yaml:"AliCloudConfig"`
	Vault                            *VaultConfig             `yaml:"VaultConfig"`
	VaultBackupBucket                *VaultBackupBucketConfig `yaml:"vaultBackupBucket"`
}

type ExportedTagsOnMetrics map[string][]string

type Metric struct {
	Name                   string   `yaml:"name"`
	Statistics             []string `yaml:"statistics"`
	Period                 int32    `yaml:"period"`
	Length                 int64    `yaml:"length"`
	Delay                  int64    `yaml:"delay"`
	NilToZero              *bool    `yaml:"nilToZero"`
	AddCloudwatchTimestamp *bool    `yaml:"addCloudwatchTimestamp"`
}

type CloudWatchMetricsConf struct {
	ExportedTagsOnMetrics ExportedTagsOnMetrics `yaml:"exportedTagsOnMetrics"`
	Jobs                  []*Job                `yaml:"jobs"`
}

type Dimension struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type Tag struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
}

type Job struct {
	Type                      string    `yaml:"type"`
	SearchTags                []Tag     `yaml:"searchTags"`
	CustomTags                []Tag     `yaml:"customTags"`
	DimensionNameRequirements []string  `yaml:"dimensionNameRequirements"`
	Metrics                   []*Metric `yaml:"metrics"`
	Length                    int64     `yaml:"length"`
	Delay                     int64     `yaml:"delay"`
	Period                    int64     `yaml:"period"`
	RoundingPeriod            *int32    `yaml:"roundingPeriod"`
	Statistics                []string  `yaml:"statistics"`
	AddCloudwatchTimestamp    *bool     `yaml:"addCloudwatchTimestamp"`
	NilToZero                 *bool     `yaml:"nilToZero"`
}

func ReadConf(filename string) (*Config, error) {
	if !fileExists(filename) {
		log.Fatal("config file not exist.")
	}
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	c := &Config{}
	err = yaml.Unmarshal(buf, c)
	if err != nil {
		return nil, fmt.Errorf("in file %q: %v", filename, err)
	}
	return c, nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
