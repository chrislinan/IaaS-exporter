package monitor

import "github.com/aws/aws-sdk-go-v2/aws"

type serviceFilter struct {
	Namespace        string
	Alias            string
	IgnoreLength     bool
	ResourceFilters  []string
	DimensionRegexps []*string
}

type serviceConfig []serviceFilter

func (sc serviceConfig) GetService(serviceType string) *serviceFilter {
	for _, sf := range sc {
		if sf.Alias == serviceType || sf.Namespace == serviceType {
			return &sf
		}
	}
	return nil
}

var (
	SupportedServices = serviceConfig{
		{
			Namespace: "AWS/ApplicationELB",
			Alias:     "alb",
			ResourceFilters: []string{
				"elasticloadbalancing:loadbalancer/app",
				"elasticloadbalancing:targetgroup",
			},
			DimensionRegexps: []*string{
				aws.String(":(?P<TargetGroup>targetgroup/.+)"),
				aws.String(":loadbalancer/(?P<LoadBalancer>.+)$"),
			},
		},
		{
			Namespace: "AWS/EBS",
			Alias:     "ebs",
			ResourceFilters: []string{
				"ec2:volume",
			},
			DimensionRegexps: []*string{
				aws.String("volume/(?P<VolumeId>[^/]+)"),
			},
		},
		{
			Namespace: "AWS/EC2",
			Alias:     "ec2",
			ResourceFilters: []string{
				"ec2:instance",
			},
			DimensionRegexps: []*string{
				aws.String("instance/(?P<InstanceId>[^/]+)"),
			},
		},
		{
			Namespace: "AWS/ELB",
			Alias:     "elb",
			ResourceFilters: []string{
				"elasticloadbalancing:loadbalancer",
			},
			DimensionRegexps: []*string{
				aws.String(":loadbalancer/(?P<LoadBalancerName>.+)$"),
			},
		}, {
			Namespace: "AWS/NATGateway",
			Alias:     "ngw",
			ResourceFilters: []string{
				"ec2:natgateway",
			},
			DimensionRegexps: []*string{
				aws.String("natgateway/(?P<NatGatewayId>[^/]+)"),
			},
		}, {
			Namespace: "AWS/NetworkELB",
			Alias:     "nlb",
			ResourceFilters: []string{
				"elasticloadbalancing:loadbalancer/net",
				"elasticloadbalancing:targetgroup",
			},
			DimensionRegexps: []*string{
				aws.String(":(?P<TargetGroup>targetgroup/.+)"),
				aws.String(":loadbalancer/(?P<LoadBalancer>.+)$"),
			},
		}, {
			Namespace: "AWS/Route53",
			Alias:     "route53",
			ResourceFilters: []string{
				"route53",
			},
			DimensionRegexps: []*string{
				aws.String(":healthcheck/(?P<HealthCheckId>[^/]+)"),
			},
		}, {
			Namespace:    "AWS/S3",
			Alias:        "s3",
			IgnoreLength: true,
			ResourceFilters: []string{
				"s3",
			},
			DimensionRegexps: []*string{
				aws.String("(?P<BucketName>[^:]+)$"),
			},
		},
	}
)
