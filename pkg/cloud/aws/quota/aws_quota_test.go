package quota

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cloudwatchType "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Type "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing"
	elbType "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2Type "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
	quotaType "github.com/aws/aws-sdk-go-v2/service/servicequotas/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/common"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
	"net/http"
	"testing"
	"time"
)

type MockIamClient struct {
}

func (m *MockIamClient) ListAccountAliases(ctx context.Context, params *iam.ListAccountAliasesInput, optFns ...func(*iam.Options)) (*iam.ListAccountAliasesOutput, error) {
	return &iam.ListAccountAliasesOutput{
		AccountAliases: []string{"hdl", "hana"},
	}, nil
}

type MockQuotaClient struct {
}

func (m *MockQuotaClient) NewServiceQuotaPager(params *servicequotas.ListServiceQuotasInput) ListServiceQuotasPager {
	// Ensure just get one ServiceQuotaPager, otherwise Prometheus will report an error.
	if *params.ServiceCode != "ebs" {
		return &MockServiceQuotaPager{}
	}
	pager := &MockServiceQuotaPager{
		Pages: []*servicequotas.ListServiceQuotasOutput{
			{
				Quotas: []quotaType.ServiceQuota{
					{
						QuotaCode: aws.String("L-43DA4232"),
						QuotaName: aws.String("Running On-Demand Standard (A, C, D, H, I, M, R, T, Z) instances"),
						UsageMetric: &quotaType.MetricInfo{
							MetricDimensions: map[string]string{
								"mock_key1": "mock_value1",
								"mock_key2": "mock_value2",
							},
							MetricName:      aws.String("mock_name1"),
							MetricNamespace: aws.String("mock_namespace1"),
						},
						Value: aws.Float64(20000),
					},
					{
						QuotaCode: aws.String("L-7295265B"),
						QuotaName: aws.String("Running On-Demand X instances"),
						UsageMetric: &quotaType.MetricInfo{
							MetricDimensions: map[string]string{
								"mock_key1": "mock_value1",
								"mock_key2": "mock_value2",
							},
							MetricName:      aws.String("mock_name2"),
							MetricNamespace: aws.String("mock_namespace2"),
						},
						Value: aws.Float64(6000),
					},
					{
						QuotaCode: aws.String("L-1216C47A"),
						QuotaName: aws.String("Running On-Demand High Memory instances"),
						UsageMetric: &quotaType.MetricInfo{
							MetricDimensions: map[string]string{
								"mock_key1": "mock_value1",
								"mock_key2": "mock_value2",
							},
							MetricName:      aws.String("mock_name3"),
							MetricNamespace: aws.String("mock_namespace3"),
						},
						Value: aws.Float64(900),
					},
					{
						QuotaCode: aws.String("L-0263D0A3"),
						QuotaName: aws.String("EC2-VPC Elastic IPs"),
						Value:     aws.Float64(200),
					},
				},
			},
			{
				Quotas: []quotaType.ServiceQuota{
					{
						QuotaCode: aws.String("L-E9E9831D"),
						QuotaName: aws.String("Classic Load Balancers per Region"),
						Value:     aws.Float64(600),
					},
					{
						QuotaCode: aws.String("L-69A177A2"),
						QuotaName: aws.String("Network Load Balancers per Region"),
						Value:     aws.Float64(2000),
					},
					{
						QuotaCode: aws.String("L-A84ABF80"),
						QuotaName: aws.String("Running Dedicated x2idn Hosts"),
						Value:     aws.Float64(1000),
					},
				},
			},
			{
				Quotas: []quotaType.ServiceQuota{
					{
						QuotaCode: aws.String("L-D18FCD1D"),
						QuotaName: aws.String("Storage for General Purpose SSD (gp2) volumes, in TiB"),
						Value:     aws.Float64(1200),
					},
					{
						QuotaCode: aws.String("L-589F43AA"),
						QuotaName: aws.String("Route tables per VPC"),
						Value:     aws.Float64(1000),
					},
					{
						QuotaCode: aws.String("L-F678F1CE"),
						QuotaName: aws.String("VPCs per Region"),
						Value:     aws.Float64(2000),
					},
					{
						QuotaCode: aws.String("L-FE5A380F"),
						QuotaName: aws.String("NAT gateways per Availability Zone"),
						Value:     aws.Float64(200),
					},
				},
			},
		},
	}
	return pager
}

type MockServiceQuotaPager struct {
	PageNum int
	Pages   []*servicequotas.ListServiceQuotasOutput
}

func (m *MockServiceQuotaPager) HasMorePages() bool {
	return m.PageNum < len(m.Pages)
}

func (m *MockServiceQuotaPager) NextPage(ctx context.Context, f ...func(*servicequotas.Options)) (output *servicequotas.ListServiceQuotasOutput, err error) {
	if m.PageNum >= len(m.Pages) {
		return nil, fmt.Errorf("no more pages")
	}
	output = m.Pages[m.PageNum]
	m.PageNum++
	return output, nil
}

type MockCloudWatchClient struct {
}

func (m *MockCloudWatchClient) GetMetricStatistics(ctx context.Context, params *cloudwatch.GetMetricStatisticsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricStatisticsOutput, error) {
	input := aws.ToString(params.MetricName)
	switch input {
	case "mock_name1":
		{
			return &cloudwatch.GetMetricStatisticsOutput{
				Datapoints: []cloudwatchType.Datapoint{
					{
						Maximum: aws.Float64(8000),
					},
				},
			}, nil
		}
	case "mock_name2":
		{
			return &cloudwatch.GetMetricStatisticsOutput{
				Datapoints: []cloudwatchType.Datapoint{
					{
						Maximum: aws.Float64(300),
					},
				},
			}, nil
		}
	case "mock_name3":
		{
			return &cloudwatch.GetMetricStatisticsOutput{
				Datapoints: []cloudwatchType.Datapoint{
					{
						Maximum: aws.Float64(90),
					},
				},
			}, nil
		}
	}
	return nil, nil
}

type MockElbClient struct {
}

func (m *MockElbClient) NewElbLoadBalancersPager(params *elb.DescribeLoadBalancersInput) DescribeElbLoadBalancersPager {
	pager := &MockElbLoadBalancersPager{
		Pages: []*elb.DescribeLoadBalancersOutput{
			{
				LoadBalancerDescriptions: []elbType.LoadBalancerDescription{
					{
						LoadBalancerName: aws.String("mock"),
					},
					{
						LoadBalancerName: aws.String("mock"),
					},
				},
			},
			{
				LoadBalancerDescriptions: []elbType.LoadBalancerDescription{
					{
						LoadBalancerName: aws.String("mock"),
					},
				},
			},
		},
	}
	return pager
}

type MockElbLoadBalancersPager struct {
	PageNum int
	Pages   []*elb.DescribeLoadBalancersOutput
}

func (m *MockElbLoadBalancersPager) HasMorePages() bool {
	return m.PageNum < len(m.Pages)
}

func (m *MockElbLoadBalancersPager) NextPage(ctx context.Context, f ...func(*elb.Options)) (output *elb.DescribeLoadBalancersOutput, err error) {
	if m.PageNum >= len(m.Pages) {
		return nil, fmt.Errorf("no more pages")
	}
	output = m.Pages[m.PageNum]
	m.PageNum++
	return output, nil
}

type MockElbv2Client struct {
}

func (m *MockElbv2Client) NewElbv2LoadBalancersPager(params *elbv2.DescribeLoadBalancersInput) DescribeElbv2LoadBalancersPager {
	pager := &MockElbv2LoadBalancersPager{
		Pages: []*elbv2.DescribeLoadBalancersOutput{
			{
				LoadBalancers: []elbv2Type.LoadBalancer{
					{
						LoadBalancerName: aws.String("mock"),
					},
					{
						LoadBalancerName: aws.String("mock"),
					},
					{
						LoadBalancerName: aws.String("mock"),
					},
				},
			},
		},
	}
	return pager
}

type MockElbv2LoadBalancersPager struct {
	PageNum int
	Pages   []*elbv2.DescribeLoadBalancersOutput
}

func (m *MockElbv2LoadBalancersPager) HasMorePages() bool {
	return m.PageNum < len(m.Pages)
}

func (m *MockElbv2LoadBalancersPager) NextPage(ctx context.Context, f ...func(*elbv2.Options)) (output *elbv2.DescribeLoadBalancersOutput, err error) {
	if m.PageNum >= len(m.Pages) {
		return nil, fmt.Errorf("no more pages")
	}
	output = m.Pages[m.PageNum]
	m.PageNum++
	return output, nil
}

type MockEc2Client struct {
}

func (m *MockEc2Client) NewVolumesPager(params *ec2.DescribeVolumesInput) DescribeVolumesPager {
	pager := &MockVolumesPager{
		Pages: []*ec2.DescribeVolumesOutput{
			{
				Volumes: []ec2Type.Volume{
					{
						Size: aws.Int32(1024),
					},
					{
						Size: aws.Int32(2048),
					},
				},
			},
		},
	}
	return pager
}

func (m *MockEc2Client) NewRouteTablesPager(params *ec2.DescribeRouteTablesInput) DescribeRouteTablesPager {
	pager := &MockRouteTablesPager{
		Pages: []*ec2.DescribeRouteTablesOutput{
			{
				RouteTables: []ec2Type.RouteTable{
					{
						RouteTableId: aws.String("mock"),
					},
					{
						RouteTableId: aws.String("mock"),
					},
				},
			},
		},
	}
	return pager
}

func (m *MockEc2Client) NewVpcsPager(params *ec2.DescribeVpcsInput) DescribeVpcsPager {
	pager := &MockVpcsPager{
		Pages: []*ec2.DescribeVpcsOutput{
			{
				Vpcs: []ec2Type.Vpc{
					{
						VpcId: aws.String("mock"),
					},
					{
						VpcId: aws.String("mock"),
					},
				},
			},
		},
	}
	return pager
}

func (m *MockEc2Client) DescribeAddresses(ctx context.Context, params *ec2.DescribeAddressesInput) (*ec2.DescribeAddressesOutput, error) {
	return &ec2.DescribeAddressesOutput{
		Addresses: []ec2Type.Address{
			{
				PublicIp: aws.String("mock"),
			},
		},
	}, nil
}

func (m *MockEc2Client) DescribeHosts(ctx context.Context, params *ec2.DescribeHostsInput) (*ec2.DescribeHostsOutput, error) {
	return &ec2.DescribeHostsOutput{}, nil
}

func (m *MockEc2Client) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error) {
	return &ec2.DescribeSubnetsOutput{
		Subnets: []ec2Type.Subnet{
			{
				SubnetId:         aws.String("Mock_SubnetId"),
				AvailabilityZone: aws.String("Mock"),
			},
			{
				SubnetId:         aws.String("Mock_SubnetId"),
				AvailabilityZone: aws.String("Mock"),
			},
			{
				SubnetId:         aws.String("Mock_SubnetId"),
				AvailabilityZone: aws.String("Mock2"),
			},
		},
	}, nil
}

func (m *MockEc2Client) DescribeNatGateways(ctx context.Context, params *ec2.DescribeNatGatewaysInput) (*ec2.DescribeNatGatewaysOutput, error) {
	return &ec2.DescribeNatGatewaysOutput{
		NatGateways: []ec2Type.NatGateway{
			{
				SubnetId: aws.String("Mock_SubnetId"),
			},
		},
	}, nil
}

type MockVolumesPager struct {
	PageNum int
	Pages   []*ec2.DescribeVolumesOutput
}

func (m *MockVolumesPager) HasMorePages() bool {
	return m.PageNum < len(m.Pages)
}

func (m *MockVolumesPager) NextPage(ctx context.Context, f ...func(*ec2.Options)) (output *ec2.DescribeVolumesOutput, err error) {
	if m.PageNum >= len(m.Pages) {
		return nil, fmt.Errorf("no more pages")
	}
	output = m.Pages[m.PageNum]
	m.PageNum++
	return output, nil
}

type MockRouteTablesPager struct {
	PageNum int
	Pages   []*ec2.DescribeRouteTablesOutput
}

func (m *MockRouteTablesPager) HasMorePages() bool {
	return m.PageNum < len(m.Pages)
}

func (m *MockRouteTablesPager) NextPage(ctx context.Context, f ...func(*ec2.Options)) (output *ec2.DescribeRouteTablesOutput, err error) {
	if m.PageNum >= len(m.Pages) {
		return nil, fmt.Errorf("no more pages")
	}
	output = m.Pages[m.PageNum]
	m.PageNum++
	return output, nil
}

type MockVpcsPager struct {
	PageNum int
	Pages   []*ec2.DescribeVpcsOutput
}

func (m *MockVpcsPager) HasMorePages() bool {
	return m.PageNum < len(m.Pages)
}

func (m *MockVpcsPager) NextPage(ctx context.Context, f ...func(*ec2.Options)) (output *ec2.DescribeVpcsOutput, err error) {
	if m.PageNum >= len(m.Pages) {
		return nil, fmt.Errorf("no more pages")
	}
	output = m.Pages[m.PageNum]
	m.PageNum++
	return output, nil
}

type MockStsClient struct {
}

func (m *MockStsClient) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Account: aws.String("dummy_account"),
	}, nil
}

type MockQuotaCache struct {
}

func (m *MockQuotaCache) Set(k string, x interface{}, d time.Duration) {}

func (m *MockQuotaCache) Items() map[string]cache.Item {
	var result = map[string]cache.Item{
		"1": {
			Expiration: 0,
			Object: &common.QuotaResult{
				QuotaCode:    "code",
				QuotaName:    "name",
				LimitValue:   100,
				CurrentValue: 30,
			},
		},
	}
	return result
}

func TestAwsQuota(t *testing.T) {
	uri := "/metrics"
	cred := vault.CloudCredentials{
		vault.AwsAccessKeyID:     "accessKeyID",
		vault.AwsSecretAccessKey: "secretAccessKey",
	}
	vaultBackupBucket := config.VaultBackupBucketConfig{
		Bucket: "mock_bucket",
		Prefix: "mock_prefix",
	}
	conf := &config.Config{
		VaultBackupBucket: &vaultBackupBucket,
		Region:            "eu-central-1",
	}
	common.QuotaCache = &MockQuotaCache{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		quotaCollector := NewMetricsCollectorAwsQuota(conf, cred, &log.Logger{})
		registry := prometheus.NewRegistry()
		quotaCollector.quotaClient = &MockQuotaClient{}
		quotaCollector.iamClient = &MockIamClient{}
		quotaCollector.cloudwatchClient = &MockCloudWatchClient{}
		quotaCollector.elbClient = &MockElbClient{}
		quotaCollector.elbv2Client = &MockElbv2Client{}
		quotaCollector.ec2Client = &MockEc2Client{}
		quotaCollector.stsClient = &MockStsClient{}
		quotaCollector.Scrape(context.TODO())
		registry.MustRegister(quotaCollector)
		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	})

	aliaClient := &MockIamClient{}
	accountAlias, _ := aliaClient.ListAccountAliases(context.TODO(), &iam.ListAccountAliasesInput{})
	_ = accountAlias.AccountAliases[0]

	assert.HTTPBodyContains(t, handler, "GET", uri, nil, "cpe_quota_current{AccountAlias=\"hdl\",AccountID=\"dummy_account\",QuotaCode=\"code\",QuotaName=\"\",Region=\"eu-central-1\",ServiceCode=\"\",ServiceName=\"\",Unit=\"\"} 30")
	assert.HTTPBodyContains(t, handler, "GET", uri, nil, "cpe_quota_limit{AccountAlias=\"hdl\",AccountID=\"dummy_account\",QuotaCode=\"code\",QuotaName=\"\",Region=\"eu-central-1\",ServiceCode=\"\",ServiceName=\"\",Unit=\"\"} 100")

}
