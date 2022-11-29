// Code generated by smithy-go-codegen DO NOT EDIT.

package elasticloadbalancing

import (
	"context"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// Removes the specified subnets from the set of configured subnets for the load
// balancer. After a subnet is removed, all EC2 instances registered with the load
// balancer in the removed subnet go into the OutOfService state. Then, the load
// balancer balances the traffic among the remaining routable subnets.
func (c *Client) DetachLoadBalancerFromSubnets(ctx context.Context, params *DetachLoadBalancerFromSubnetsInput, optFns ...func(*Options)) (*DetachLoadBalancerFromSubnetsOutput, error) {
	if params == nil {
		params = &DetachLoadBalancerFromSubnetsInput{}
	}

	result, metadata, err := c.invokeOperation(ctx, "DetachLoadBalancerFromSubnets", params, optFns, c.addOperationDetachLoadBalancerFromSubnetsMiddlewares)
	if err != nil {
		return nil, err
	}

	out := result.(*DetachLoadBalancerFromSubnetsOutput)
	out.ResultMetadata = metadata
	return out, nil
}

// Contains the parameters for DetachLoadBalancerFromSubnets.
type DetachLoadBalancerFromSubnetsInput struct {

	// The name of the load balancer.
	//
	// This member is required.
	LoadBalancerName *string

	// The IDs of the subnets.
	//
	// This member is required.
	Subnets []string

	noSmithyDocumentSerde
}

// Contains the output of DetachLoadBalancerFromSubnets.
type DetachLoadBalancerFromSubnetsOutput struct {

	// The IDs of the remaining subnets for the load balancer.
	Subnets []string

	// Metadata pertaining to the operation's result.
	ResultMetadata middleware.Metadata

	noSmithyDocumentSerde
}

func (c *Client) addOperationDetachLoadBalancerFromSubnetsMiddlewares(stack *middleware.Stack, options Options) (err error) {
	err = stack.Serialize.Add(&awsAwsquery_serializeOpDetachLoadBalancerFromSubnets{}, middleware.After)
	if err != nil {
		return err
	}
	err = stack.Deserialize.Add(&awsAwsquery_deserializeOpDetachLoadBalancerFromSubnets{}, middleware.After)
	if err != nil {
		return err
	}
	if err = addSetLoggerMiddleware(stack, options); err != nil {
		return err
	}
	if err = awsmiddleware.AddClientRequestIDMiddleware(stack); err != nil {
		return err
	}
	if err = smithyhttp.AddComputeContentLengthMiddleware(stack); err != nil {
		return err
	}
	if err = addResolveEndpointMiddleware(stack, options); err != nil {
		return err
	}
	if err = v4.AddComputePayloadSHA256Middleware(stack); err != nil {
		return err
	}
	if err = addRetryMiddlewares(stack, options); err != nil {
		return err
	}
	if err = addHTTPSignerV4Middleware(stack, options); err != nil {
		return err
	}
	if err = awsmiddleware.AddRawResponseToMetadata(stack); err != nil {
		return err
	}
	if err = awsmiddleware.AddRecordResponseTiming(stack); err != nil {
		return err
	}
	if err = addClientUserAgent(stack); err != nil {
		return err
	}
	if err = smithyhttp.AddErrorCloseResponseBodyMiddleware(stack); err != nil {
		return err
	}
	if err = smithyhttp.AddCloseResponseBodyMiddleware(stack); err != nil {
		return err
	}
	if err = addOpDetachLoadBalancerFromSubnetsValidationMiddleware(stack); err != nil {
		return err
	}
	if err = stack.Initialize.Add(newServiceMetadataMiddleware_opDetachLoadBalancerFromSubnets(options.Region), middleware.Before); err != nil {
		return err
	}
	if err = addRequestIDRetrieverMiddleware(stack); err != nil {
		return err
	}
	if err = addResponseErrorMiddleware(stack); err != nil {
		return err
	}
	if err = addRequestResponseLogging(stack, options); err != nil {
		return err
	}
	return nil
}

func newServiceMetadataMiddleware_opDetachLoadBalancerFromSubnets(region string) *awsmiddleware.RegisterServiceMetadata {
	return &awsmiddleware.RegisterServiceMetadata{
		Region:        region,
		ServiceID:     ServiceID,
		SigningName:   "elasticloadbalancing",
		OperationName: "DetachLoadBalancerFromSubnets",
	}
}
