package monitor

import (
	"github.com/prometheus/client_golang/prometheus"
	"regexp"
	"strings"
	"time"
)

type LabelSet map[string]struct{}

var splitRegexp = regexp.MustCompile(`([a-z0-9])([A-Z])`)

var replacer = strings.NewReplacer(
	" ", "_",
	",", "_",
	"\t", "_",
	"/", "_",
	"\\", "_",
	".", "_",
	"-", "_",
	":", "_",
	"=", "_",
	"“", "_",
	"@", "_",
	"<", "_",
	">", "_",
	"%", "_percent",
)

type PrometheusMetric struct {
	name             *string
	labels           map[string]string
	value            *float64
	includeTimestamp bool
	timestamp        time.Time
}

func createDesc(metric *PrometheusMetric) *prometheus.Desc {
	return prometheus.NewDesc(
		*metric.name,
		"Help is not implemented yet.",
		nil,
		metric.labels,
	)
}

func createMetric(metric *PrometheusMetric) prometheus.Metric {
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        *metric.name,
		Help:        "Help is not implemented yet.",
		ConstLabels: metric.labels,
	})
	gauge.Set(*metric.value)

	if !metric.includeTimestamp {
		return gauge
	}

	return prometheus.NewMetricWithTimestamp(metric.timestamp, gauge)
}

func promString(text string) string {
	text = splitRegexp.ReplaceAllString(text, `$1.$2`)
	return strings.ToLower(replacer.Replace(text))
}

func recordLabelsForMetric(metricName string, promLabels map[string]string, observedMetricLabels map[string]LabelSet) map[string]LabelSet {
	if _, ok := observedMetricLabels[metricName]; !ok {
		observedMetricLabels[metricName] = make(LabelSet)
	}
	for label := range promLabels {
		if _, ok := observedMetricLabels[metricName][label]; !ok {
			observedMetricLabels[metricName][label] = struct{}{}
		}
	}

	return observedMetricLabels
}

func createPrometheusLabels(cwd *cloudwatchData) map[string]string {
	labels := make(map[string]string)
	labels["name"] = *cwd.ID
	labels["region"] = *cwd.Region
	labels["account_id"] = *cwd.AccountId

	// Inject the sfn name back as a label
	for _, dimension := range cwd.Dimensions {
		labels["dimension_"+promString(*dimension.Name)] = *dimension.Value
	}

	for _, label := range cwd.CustomTags {
		labels["custom_tag_"+promString(label.Key)] = label.Value
	}
	for _, tag := range cwd.Tags {
		labels["tag_"+promString(tag.Key)] = tag.Value
	}

	return labels
}
