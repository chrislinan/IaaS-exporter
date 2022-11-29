package monitor

import (
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"regexp"
)

type taggedResource struct {
	ARN       string
	Namespace string
	Region    string
	Tags      []config.Tag
}

func (r taggedResource) filterThroughTags(filterTags []config.Tag) bool {
	tagMatches := 0
	for _, resourceTag := range r.Tags {
		for _, filterTag := range filterTags {
			if resourceTag.Key == filterTag.Key {
				r, _ := regexp.Compile(filterTag.Value)
				if r.MatchString(resourceTag.Value) {
					tagMatches++
				}
			}
		}
	}
	return tagMatches == len(filterTags)
}

func (r taggedResource) metricTags(tagsOnMetrics config.ExportedTagsOnMetrics) []config.Tag {
	tags := make([]config.Tag, 0)
	for _, tagName := range tagsOnMetrics[r.Namespace] {
		tag := config.Tag{
			Key: tagName,
		}
		for _, resourceTag := range r.Tags {
			if resourceTag.Key == tagName {
				tag.Value = resourceTag.Value
				break
			}
		}
		tags = append(tags, tag)
	}
	return tags
}
