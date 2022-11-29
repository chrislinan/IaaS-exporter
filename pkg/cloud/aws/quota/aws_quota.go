package quota

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConf "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cloudwatchType "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Type "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
	servicequotaType "github.com/aws/aws-sdk-go-v2/service/servicequotas/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/cloud/common"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/config"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/constant"
	"github.wdf.sap.corp/DBaaS/cloud-provider-exporter/pkg/vault"
	"golang.org/x/exp/slices"
	"math"
	"strings"
	"sync"
	"time"
)

// IQuotaClient Mock servicequotas.client and ListServiceQuotasPaginator for test
type IQuotaClient interface {
	NewServiceQuotaPager(params *servicequotas.ListServiceQuotasInput) ListServiceQuotasPager
}

type QuotaClientWrapper struct {
	client *servicequotas.Client
}

func (c *QuotaClientWrapper) NewServiceQuotaPager(params *servicequotas.ListServiceQuotasInput) ListServiceQuotasPager {
	return servicequotas.NewListServiceQuotasPaginator(c.client, params)
}

type ListServiceQuotasPager interface {
	HasMorePages() bool
	NextPage(ctx context.Context, optFns ...func(*servicequotas.Options)) (*servicequotas.ListServiceQuotasOutput, error)
}

// IIamClient Mock iam.client for test
type IIamClient interface {
	ListAccountAliases(ctx context.Context, params *iam.ListAccountAliasesInput, optFns ...func(*iam.Options)) (*iam.ListAccountAliasesOutput, error)
}

// IStsClient Mock sts.client for test
type IStsClient interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

// ICloudWatchClient Mock cloudwatch.client for test
type ICloudWatchClient interface {
	GetMetricStatistics(ctx context.Context, params *cloudwatch.GetMetricStatisticsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricStatisticsOutput, error)
}

// IElbClient Mock Elb.Client and Elb.DescribeLoadBalancersPaginator for test
type IElbClient interface {
	NewElbLoadBalancersPager(params *elb.DescribeLoadBalancersInput) DescribeElbLoadBalancersPager
}

type ElbClientWrapper struct {
	client *elb.Client
}

func (c *ElbClientWrapper) NewElbLoadBalancersPager(params *elb.DescribeLoadBalancersInput) DescribeElbLoadBalancersPager {
	return elb.NewDescribeLoadBalancersPaginator(c.client, params)
}

type DescribeElbLoadBalancersPager interface {
	HasMorePages() bool
	NextPage(ctx context.Context, optFns ...func(*elb.Options)) (*elb.DescribeLoadBalancersOutput, error)
}

// IElbv2Client Mock Elbv2.Client and Elb.DescribeLoadBalancersPaginator for test
type IElbv2Client interface {
	NewElbv2LoadBalancersPager(params *elbv2.DescribeLoadBalancersInput) DescribeElbv2LoadBalancersPager
}

type Elbv2ClientWrapper struct {
	client *elbv2.Client
}

func (c *Elbv2ClientWrapper) NewElbv2LoadBalancersPager(params *elbv2.DescribeLoadBalancersInput) DescribeElbv2LoadBalancersPager {
	return elbv2.NewDescribeLoadBalancersPaginator(c.client, params)
}

type DescribeElbv2LoadBalancersPager interface {
	HasMorePages() bool
	NextPage(ctx context.Context, optFns ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancersOutput, error)
}

// IEc2Client Mock Ec2.Client for test
type IEc2Client interface {
	NewVolumesPager(params *ec2.DescribeVolumesInput) DescribeVolumesPager
	NewRouteTablesPager(params *ec2.DescribeRouteTablesInput) DescribeRouteTablesPager
	NewVpcsPager(params *ec2.DescribeVpcsInput) DescribeVpcsPager
	DescribeAddresses(ctx context.Context, params *ec2.DescribeAddressesInput) (*ec2.DescribeAddressesOutput, error)
	DescribeHosts(ctx context.Context, params *ec2.DescribeHostsInput) (*ec2.DescribeHostsOutput, error)
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error)
	DescribeNatGateways(ctx context.Context, params *ec2.DescribeNatGatewaysInput) (*ec2.DescribeNatGatewaysOutput, error)
}

type Ec2ClientWrapper struct {
	client *ec2.Client
}

func (c *Ec2ClientWrapper) NewVolumesPager(params *ec2.DescribeVolumesInput) DescribeVolumesPager {
	return ec2.NewDescribeVolumesPaginator(c.client, params)
}

func (c *Ec2ClientWrapper) NewRouteTablesPager(params *ec2.DescribeRouteTablesInput) DescribeRouteTablesPager {
	return ec2.NewDescribeRouteTablesPaginator(c.client, params)
}

func (c *Ec2ClientWrapper) NewVpcsPager(params *ec2.DescribeVpcsInput) DescribeVpcsPager {
	return ec2.NewDescribeVpcsPaginator(c.client, params)
}

func (c *Ec2ClientWrapper) DescribeAddresses(ctx context.Context, params *ec2.DescribeAddressesInput) (*ec2.DescribeAddressesOutput, error) {
	return c.client.DescribeAddresses(ctx, params)
}

func (c *Ec2ClientWrapper) DescribeHosts(ctx context.Context, params *ec2.DescribeHostsInput) (*ec2.DescribeHostsOutput, error) {
	return c.client.DescribeHosts(ctx, params)
}

func (c *Ec2ClientWrapper) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error) {
	return c.client.DescribeSubnets(ctx, params)
}

func (c *Ec2ClientWrapper) DescribeNatGateways(ctx context.Context, params *ec2.DescribeNatGatewaysInput) (*ec2.DescribeNatGatewaysOutput, error) {
	return c.client.DescribeNatGateways(ctx, params)
}

type DescribeVolumesPager interface {
	HasMorePages() bool
	NextPage(ctx context.Context, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
}

type DescribeRouteTablesPager interface {
	HasMorePages() bool
	NextPage(ctx context.Context, optFns ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error)
}

type DescribeVpcsPager interface {
	HasMorePages() bool
	NextPage(ctx context.Context, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
}

var (
	accountID       string
	account         string
	waitGroup       sync.WaitGroup
	services        = []string{"ec2", "vpc", "elasticloadbalancing", "ebs"}
	serviceQuotaMap = make(map[string]servicequotaType.ServiceQuota)
	quotaCodeList   = []string{"L-43DA4232", "L-7295265B", "L-1216C47A",
		"L-D18FCD1D", "L-589F43AA", "L-F678F1CE",
		"L-0263D0A3", "L-E9E9831D", "L-FE5A380F",
		"L-A84ABF80", "L-69A177A2"}
)

type MetricsCollectorAwsQuota struct {
	quotaClient      IQuotaClient
	cloudwatchClient ICloudWatchClient
	ec2Client        IEc2Client
	elbClient        IElbClient
	elbv2Client      IElbv2Client
	iamClient        IIamClient
	stsClient        IStsClient
	conf             *config.Config
	log              log.FieldLogger
}

func (m *MetricsCollectorAwsQuota) initialQuotaList(ctx context.Context, logger log.FieldLogger) {
	accountAlias, err := m.iamClient.ListAccountAliases(ctx, &iam.ListAccountAliasesInput{})
	if err != nil {
		logger.Errorf("Error while getting accountAlias: ", err)
	}
	if len(accountAlias.AccountAliases) > 0 {
		account = accountAlias.AccountAliases[0]
	}
	id, err := m.stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		logger.Errorf("Error while getting accountID: ", err)
	}
	accountID = aws.ToString(id.Account)

	for _, service := range services {
		paginator := m.quotaClient.NewServiceQuotaPager(&servicequotas.ListServiceQuotasInput{ServiceCode: aws.String(service)})
		for paginator.HasMorePages() {
			out, err := paginator.NextPage(ctx)
			if err != nil {
				logger.Errorf("Error while getting next service page: ", err)
			}
			for _, q := range out.Quotas {
				quotaCode := aws.ToString(q.QuotaCode)
				if slices.Contains(quotaCodeList, quotaCode) {
					serviceQuotaMap[quotaCode] = q
					m.log.WithFields(log.Fields{"serviceName": q.ServiceName, "serviceCode": q.ServiceCode, "quotaName": q.QuotaName, "quotaCode": quotaCode, "limit": q.Value}).Infof("retrieve limit value")
				}
			}
		}
	}
}

func NewMetricsCollectorAwsQuota(config *config.Config, cred vault.CloudCredentials, logger log.FieldLogger) *MetricsCollectorAwsQuota {
	m := &MetricsCollectorAwsQuota{}
	m.conf = config
	m.log = logger
	appCred := aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(cred[vault.AwsAccessKeyID], cred[vault.AwsSecretAccessKey], ""))
	cfg, err := awsConf.LoadDefaultConfig(context.TODO(), awsConf.WithCredentialsProvider(appCred), awsConf.WithRegion(m.conf.Region))
	if err != nil {
		m.log.Errorf("Error while loading default config: ", err)
	}
	m.log.Infof("Initialize different AWS clients")
	m.quotaClient = &QuotaClientWrapper{client: servicequotas.NewFromConfig(cfg)}
	m.cloudwatchClient = cloudwatch.NewFromConfig(cfg)
	m.ec2Client = &Ec2ClientWrapper{client: ec2.NewFromConfig(cfg)}
	m.elbClient = &ElbClientWrapper{client: elb.NewFromConfig(cfg)}
	m.elbv2Client = &Elbv2ClientWrapper{client: elbv2.NewFromConfig(cfg)}
	m.iamClient = iam.NewFromConfig(cfg)
	m.stsClient = sts.NewFromConfig(cfg)
	common.InitQuotaGaugeVec("AWS", []string{constant.LabelRegion, constant.LabelServiceName, constant.LabelServiceCode, constant.LabelQuotaName, constant.LabelQuotaCode, constant.LabelAccountID, constant.LabelAccountAlias, constant.LabelUnit})
	return m
}

func (m *MetricsCollectorAwsQuota) Describe(ch chan<- *prometheus.Desc) {
	common.QuotaLimit.Describe(ch)
	common.QuotaCurrent.Describe(ch)
}

func (m *MetricsCollectorAwsQuota) Scrape(ctx context.Context) {
	m.log.Infof("Start collect AWS metrics")
	m.initialQuotaList(ctx, m.log)
	for _, qCode := range quotaCodeList {
		waitGroup.Add(1)
		go func(qCode string, q servicequotaType.ServiceQuota) {
			defer waitGroup.Done()
			var currentValue, limitValue float64
			switch qCode {
			case "L-43DA4232", "L-7295265B", "L-1216C47A": // EC2: Running On-Demand Standard (A, C, D, H, I, M, R, T, Z) instances, EC2: Running On-Demand X instances, EC2: Running On-Demand High Memory instances
				{
					m.log.Infof("Start collect metrics - EC2: %v", aws.ToString(q.QuotaName))
					var dimensions []cloudwatchType.Dimension
					for k, v := range q.UsageMetric.MetricDimensions {
						dimensions = append(dimensions, cloudwatchType.Dimension{Name: aws.String(k), Value: aws.String(v)})
					}
					input := &cloudwatch.GetMetricStatisticsInput{
						MetricName: q.UsageMetric.MetricName,
						Namespace:  q.UsageMetric.MetricNamespace,
						Statistics: []cloudwatchType.Statistic{cloudwatchType.Statistic(aws.ToString(q.UsageMetric.MetricStatisticRecommendation))},
						Dimensions: dimensions,
						EndTime:    aws.Time(time.Now().UTC()),
						StartTime:  aws.Time(time.Now().Add(-time.Duration(1) * time.Hour)),
						Period:     aws.Int32(3600),
					}
					stats, err := m.cloudwatchClient.GetMetricStatistics(ctx, input)
					if err != nil {
						m.log.Errorf("Error while getting metric statistics: ", err)
					}
					if stats.Datapoints != nil && len(stats.Datapoints) > 0 {
						currentValue = aws.ToFloat64(stats.Datapoints[0].Maximum)
					} else {
						currentValue = 0
					}
					limitValue = aws.ToFloat64(q.Value)
					result := &common.QuotaResult{QuotaCode: aws.ToString(q.QuotaCode), QuotaName: aws.ToString(q.QuotaName), LimitValue: limitValue, CurrentValue: currentValue, Unit: aws.ToString(q.Unit)}
					common.QuotaCache.Set(aws.ToString(q.QuotaCode), result, cache.DefaultExpiration)
				}
			case "L-D18FCD1D": // EBS: General Purpose (SSD) volume storage
				{
					m.log.Infof("Start collect metrics - EBS: General Purpose (SSD) volume storage")
					filters := []ec2Type.Filter{{
						Name:   aws.String("volume-type"),
						Values: []string{"gp2"},
					}}
					input := &ec2.DescribeVolumesInput{Filters: filters}
					describeVolumePages := m.ec2Client.NewVolumesPager(input)
					var usedQuotaGib int32
					for describeVolumePages.HasMorePages() {
						out, err := describeVolumePages.NextPage(ctx)
						if err != nil {
							m.log.Errorf("Error while getting next volume page: ", err)
						}
						for _, volume := range out.Volumes {
							usedQuotaGib += aws.ToInt32(volume.Size)
						}
					}
					currentValue = math.Round(float64(usedQuotaGib / 1024))
					limitValue = aws.ToFloat64(q.Value)
					result := &common.QuotaResult{QuotaCode: aws.ToString(q.QuotaCode), QuotaName: aws.ToString(q.QuotaName), LimitValue: limitValue, CurrentValue: currentValue, Unit: aws.ToString(q.Unit)}
					common.QuotaCache.Set(aws.ToString(q.QuotaCode), result, cache.DefaultExpiration)
				}
			case "L-589F43AA": // VPC: Route tables per VPC
				{
					m.log.Infof("Start collect metrics - VPC: Route tables per VPC")
					if !strings.Contains(account, "hdl") {
						break
					}
					input := &ec2.DescribeRouteTablesInput{}
					describeRouteTablePage := m.ec2Client.NewRouteTablesPager(input)
					var routeTablesPerVpc int
					for describeRouteTablePage.HasMorePages() {
						out, err := describeRouteTablePage.NextPage(ctx)
						if err != nil {
							m.log.Errorf("Error while getting next route table page: ", err)
						}
						routeTablesPerVpc += len(out.RouteTables)
					}
					currentValue = float64(routeTablesPerVpc)
					limitValue = aws.ToFloat64(q.Value)
					result := &common.QuotaResult{QuotaCode: aws.ToString(q.QuotaCode), QuotaName: aws.ToString(q.QuotaName), LimitValue: limitValue, CurrentValue: currentValue, Unit: aws.ToString(q.Unit)}
					common.QuotaCache.Set(aws.ToString(q.QuotaCode), result, cache.DefaultExpiration)
				}
			case "L-F678F1CE": // VPC: VPCs per Region
				{
					m.log.Infof("Start collect metrics - VPC: VPCs per Region")
					input := &ec2.DescribeVpcsInput{}
					describeVpcPage := m.ec2Client.NewVpcsPager(input)
					var vpcPerRegion int
					for describeVpcPage.HasMorePages() {
						out, err := describeVpcPage.NextPage(ctx)
						if err != nil {
							m.log.Errorf("Error while getting next Vpc page: ", err)
						}
						vpcPerRegion += len(out.Vpcs)
					}
					currentValue = float64(vpcPerRegion)
					limitValue = aws.ToFloat64(q.Value)
					result := &common.QuotaResult{QuotaCode: aws.ToString(q.QuotaCode), QuotaName: aws.ToString(q.QuotaName), LimitValue: limitValue, CurrentValue: currentValue, Unit: aws.ToString(q.Unit)}
					common.QuotaCache.Set(aws.ToString(q.QuotaCode), result, cache.DefaultExpiration)
				}
			case "L-0263D0A3": // EC2: Number of EIPs - VPC EIPs
				{
					m.log.Infof("Start collect metrics - EC2: Number of EIPs - VPC EIPs")
					out, err := m.ec2Client.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{})
					if err != nil {
						m.log.Errorf("Error while getting addresses: ", err)
					}
					currentValue = float64(len(out.Addresses))
					limitValue = aws.ToFloat64(q.Value)
					result := &common.QuotaResult{QuotaCode: aws.ToString(q.QuotaCode), QuotaName: aws.ToString(q.QuotaName), LimitValue: limitValue, CurrentValue: currentValue, Unit: aws.ToString(q.Unit)}
					common.QuotaCache.Set(aws.ToString(q.QuotaCode), result, cache.DefaultExpiration)
				}
			case "L-A84ABF80": // EC2: Running Dedicated x2idn Hosts
				{
					m.log.Infof("Start collect metrics - EC2: Running Dedicated x2idn Hosts")
					filter := ec2Type.Filter{
						Name:   aws.String("instance-type"),
						Values: []string{"x2idn*"},
					}
					input := &ec2.DescribeHostsInput{
						Filter: []ec2Type.Filter{filter},
					}
					out, err := m.ec2Client.DescribeHosts(ctx, input)
					if err != nil {
						m.log.Errorf("Error while getting hosts: ", err)
					}
					currentValue = float64(len(out.Hosts))
					limitValue = aws.ToFloat64(q.Value)
					result := &common.QuotaResult{QuotaCode: aws.ToString(q.QuotaCode), QuotaName: aws.ToString(q.QuotaName), LimitValue: limitValue, CurrentValue: currentValue, Unit: aws.ToString(q.Unit)}
					common.QuotaCache.Set(aws.ToString(q.QuotaCode), result, cache.DefaultExpiration)
				}
			case "L-69A177A2": // ELB: Network Load Balancers per Region
				{
					m.log.Infof("Start collect metrics - ELB: Network Load Balancers per Region")
					describeLoadBalancerPage := m.elbv2Client.NewElbv2LoadBalancersPager(&elbv2.DescribeLoadBalancersInput{})
					var nlbPerRegion int
					if describeLoadBalancerPage == nil {
						m.log.Errorf("Error occurred when create NewElbv2LoadBalancersPager")
						return
					}
					for describeLoadBalancerPage.HasMorePages() {
						out, err := describeLoadBalancerPage.NextPage(ctx)
						if err != nil {
							m.log.Errorf("Error while getting next load balance page: ", err)
							return
						}
						nlbPerRegion += len(out.LoadBalancers)
					}
					currentValue = float64(nlbPerRegion)
					limitValue = aws.ToFloat64(q.Value)
					result := &common.QuotaResult{QuotaCode: aws.ToString(q.QuotaCode), QuotaName: aws.ToString(q.QuotaName), LimitValue: limitValue, CurrentValue: currentValue, Unit: aws.ToString(q.Unit)}
					common.QuotaCache.Set(aws.ToString(q.QuotaCode), result, cache.DefaultExpiration)
				}
			case "L-E9E9831D": // ELB: Classic Load Balancers per Region
				{
					m.log.Infof("Start collect metrics - ELB: Classic Load Balancers per Region")
					describeClassicLoadBalancerPage := m.elbClient.NewElbLoadBalancersPager(&elb.DescribeLoadBalancersInput{})
					if describeClassicLoadBalancerPage == nil {
						m.log.Errorf("Error occurred when create NewElbLoadBalancersPager")
						return
					}
					var clbPerRegion int
					for describeClassicLoadBalancerPage.HasMorePages() {
						out, err := describeClassicLoadBalancerPage.NextPage(ctx)
						if err != nil || out == nil {
							m.log.Errorf("Error while getting next classic load balance page: ", err)
							return
						}
						clbPerRegion += len(out.LoadBalancerDescriptions)
					}
					currentValue = float64(clbPerRegion)
					limitValue = aws.ToFloat64(q.Value)
					result := &common.QuotaResult{QuotaCode: aws.ToString(q.QuotaCode), QuotaName: aws.ToString(q.QuotaName), LimitValue: limitValue, CurrentValue: currentValue, Unit: aws.ToString(q.Unit)}
					common.QuotaCache.Set(aws.ToString(q.QuotaCode), result, cache.DefaultExpiration)
				}
			case "L-FE5A380F": // VPC: NAT gateways per Availability Zone
				{
					m.log.Infof("Start collect metrics - VPC: NAT gateways per Availability Zone")
					ngwCountPerAzs := make(map[string]int32)
					subnets, err := m.ec2Client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{})
					if err != nil {
						m.log.Errorf("Error while getting subnets: ", err)
					}
					filters := []ec2Type.Filter{{
						Name:   aws.String("state"),
						Values: []string{"available"},
					}}
					natGateways, err := m.ec2Client.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{
						Filter: filters,
					})
					if err != nil {
						m.log.Errorf("Error while getting nat gateways: ", err)
					}
					for _, ngw := range natGateways.NatGateways {
						for _, subnet := range subnets.Subnets {
							if aws.ToString(subnet.SubnetId) == aws.ToString(ngw.SubnetId) {
								availabilityZone := aws.ToString(subnet.AvailabilityZone)
								if val, ok := ngwCountPerAzs[availabilityZone]; ok {
									ngwCountPerAzs[availabilityZone] = val + 1
								} else {
									ngwCountPerAzs[availabilityZone] = 0
								}
							}
						}
					}
					var usage int32 = 0
					for _, ngwCountPerAz := range ngwCountPerAzs {
						if ngwCountPerAz > usage {
							usage = ngwCountPerAz
						}
					}
					currentValue = float64(usage)
					limitValue = aws.ToFloat64(q.Value)
					result := &common.QuotaResult{QuotaCode: aws.ToString(q.QuotaCode), QuotaName: aws.ToString(q.QuotaName), LimitValue: limitValue, CurrentValue: currentValue, Unit: aws.ToString(q.Unit)}
					common.QuotaCache.Set(aws.ToString(q.QuotaCode), result, cache.DefaultExpiration)
				}
			}
		}(qCode, serviceQuotaMap[qCode])
	}
	waitGroup.Wait()
	m.log.Infof("End collect AWS metrics")
}

func (m *MetricsCollectorAwsQuota) Collect(ch chan<- prometheus.Metric) {
	m.log.Infof("Start retrieve data from cache")
	for _, item := range common.QuotaCache.Items() {
		result := item.Object.(*common.QuotaResult)
		q := serviceQuotaMap[result.QuotaCode]
		m.log.WithFields(log.Fields{"region": m.conf.Region, "serviceName": q.ServiceName, "serviceCode": q.ServiceCode, "quotaName": q.QuotaName, "quotaCode": result.QuotaCode, "accountID": accountID, "accountName": account, "current": result.CurrentValue, "limit": result.LimitValue}).Infof("retrieve data from cache")
		common.QuotaCurrent.WithLabelValues(m.conf.Region, aws.ToString(q.ServiceName), aws.ToString(q.ServiceCode), aws.ToString(q.QuotaName), result.QuotaCode, accountID, account, result.Unit).Set(result.CurrentValue)
		common.QuotaLimit.WithLabelValues(m.conf.Region, aws.ToString(q.ServiceName), aws.ToString(q.ServiceCode), aws.ToString(q.QuotaName), result.QuotaCode, accountID, account, result.Unit).Set(result.LimitValue)
	}
	common.QuotaLimit.Collect(ch)
	common.QuotaCurrent.Collect(ch)
}
