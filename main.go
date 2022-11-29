package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/common"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/factory"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
)

func main() {
	port := "8080"
	var log = &logrus.Logger{
		Out:       os.Stderr,
		Formatter: new(logrus.JSONFormatter),
		Hooks:     make(logrus.LevelHooks),
		Level:     logrus.DebugLevel,
	}
	ctx := context.Background()
	conf, err := config.ReadConf("./config.yaml")
	if err != nil {
		log.Fatal(err)
	}
	f := &factory.ExporterFactory{}

	var vaultKey = "deployment"

	exporter := f.NewExporter(conf.Provider)
	vaultClient, err := vault.NewVaultClient(conf)
	if err != nil {
		log.Fatal(err)
	}
	vaultPath := fmt.Sprintf("%s/static/%s/%s/%s", conf.Project, conf.Provider, conf.CloudProviderAccountVaultSubpath, vaultKey)
	credential, err := vaultClient.CredentialsFromPath(vaultPath, conf.Provider)
	if err != nil {
		log.Fatal(err)
	}
	common.QuotaCache = cache.New(time.Duration(conf.CacheExpiration)*time.Minute, time.Duration(conf.CacheCleanupInterval)*time.Minute)

	exporter.StartExporter(ctx, conf, credential, log)
	log.Infof("start first scraping async")
	exporter.Scrape(ctx)
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	log.Infof("server start on port: %s", port)
	go func() {
		if err = http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
			log.Errorf("An error occured: %v", err)
		}
	}()
	scrapingDuration := time.Duration(conf.ScrapingDuration) * time.Minute
	ticker := time.NewTicker(scrapingDuration)
	for {
		select {
		case <-ctx.Done():
			log.Infof("end scraping async")
			return
		case <-ticker.C:
			log.Infof("start scraping async")
			go exporter.Scrape(ctx)
		}
	}
}
