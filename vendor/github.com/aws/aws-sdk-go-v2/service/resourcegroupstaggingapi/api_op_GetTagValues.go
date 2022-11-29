// Code generated by smithy-go-codegen DO NOT EDIT.

package resourcegroupstaggingapi

import (
	"context"
	"fmt"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// Returns all tag values for the specified key that are used in the specified
// Amazon Web Services Region for the calling account. This operation supports
// pagination, where the response can be sent in multiple pages. You should check
// the PaginationToken response parameter to determine if there are additional
// results available to return. Repeat the query, passing the PaginationToken
// response parameter value as an input to the next request until you recieve a
// null value. A null value for PaginationToken indicates that there are no more
// results waiting to be returned.
func (c *Client) GetTagValues(ctx context.Context, params *GetTagValuesInput, optFns ...func(*Options)) (*GetTagValuesOutput, error) {
	if params == nil {
		params = &GetTagValuesInput{}
	}

	result, metadata, err := c.invokeOperation(ctx, "GetTagValues", params, optFns, c.addOperationGetTagValuesMiddlewares)
	if err != nil {
		return nil, err
	}

	out := result.(*GetTagValuesOutput)
	out.ResultMetadata = metadata
	return out, nil
}

type GetTagValuesInput struct {

	// Specifies the tag key for which you want to list all existing values that are
	// currently used in the specified Amazon Web Services Region for the calling
	// account.
	//
	// This member is required.
	Key *string

	// Specifies a PaginationToken response value from a previous request to indicate
	// that you want the next page of results. Leave this parameter empty in your
	// initial request.
	PaginationToken *string

	noSmithyDocumentSerde
}

type GetTagValuesOutput struct {

	// A string that indicates that there is more data available than this response
	// contains. To receive the next part of the response, specify this response value
	// as the PaginationToken value in the request for the next page.
	PaginationToken *string

	// A list of all tag values for the specified key currently used in the specified
	// Amazon Web Services Region for the calling account.
	TagValues []string

	// Metadata pertaining to the operation's result.
	ResultMetadata middleware.Metadata

	noSmithyDocumentSerde
}

func (c *Client) addOperationGetTagValuesMiddlewares(stack *middleware.Stack, options Options) (err error) {
	err = stack.Serialize.Add(&awsAwsjson11_serializeOpGetTagValues{}, middleware.After)
	if err != nil {
		return err
	}
	err = stack.Deserialize.Add(&awsAwsjson11_deserializeOpGetTagValues{}, middleware.After)
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
	if err = addOpGetTagValuesValidationMiddleware(stack); err != nil {
		return err
	}
	if err = stack.Initialize.Add(newServiceMetadataMiddleware_opGetTagValues(options.Region), middleware.Before); err != nil {
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

// GetTagValuesAPIClient is a client that implements the GetTagValues operation.
type GetTagValuesAPIClient interface {
	GetTagValues(context.Context, *GetTagValuesInput, ...func(*Options)) (*GetTagValuesOutput, error)
}

var _ GetTagValuesAPIClient = (*Client)(nil)

// GetTagValuesPaginatorOptions is the paginator options for GetTagValues
type GetTagValuesPaginatorOptions struct {
	// Set to true if pagination should stop if the service returns a pagination token
	// that matches the most recent token provided to the service.
	StopOnDuplicateToken bool
}

// GetTagValuesPaginator is a paginator for GetTagValues
type GetTagValuesPaginator struct {
	options   GetTagValuesPaginatorOptions
	client    GetTagValuesAPIClient
	params    *GetTagValuesInput
	nextToken *string
	firstPage bool
}

// NewGetTagValuesPaginator returns a new GetTagValuesPaginator
func NewGetTagValuesPaginator(client GetTagValuesAPIClient, params *GetTagValuesInput, optFns ...func(*GetTagValuesPaginatorOptions)) *GetTagValuesPaginator {
	if params == nil {
		params = &GetTagValuesInput{}
	}

	options := GetTagValuesPaginatorOptions{}

	for _, fn := range optFns {
		fn(&options)
	}

	return &GetTagValuesPaginator{
		options:   options,
		client:    client,
		params:    params,
		firstPage: true,
		nextToken: params.PaginationToken,
	}
}

// HasMorePages returns a boolean indicating whether more pages are available
func (p *GetTagValuesPaginator) HasMorePages() bool {
	return p.firstPage || (p.nextToken != nil && len(*p.nextToken) != 0)
}

// NextPage retrieves the next GetTagValues page.
func (p *GetTagValuesPaginator) NextPage(ctx context.Context, optFns ...func(*Options)) (*GetTagValuesOutput, error) {
	if !p.HasMorePages() {
		return nil, fmt.Errorf("no more pages available")
	}

	params := *p.params
	params.PaginationToken = p.nextToken

	result, err := p.client.GetTagValues(ctx, &params, optFns...)
	if err != nil {
		return nil, err
	}
	p.firstPage = false

	prevToken := p.nextToken
	p.nextToken = result.PaginationToken

	if p.options.StopOnDuplicateToken &&
		prevToken != nil &&
		p.nextToken != nil &&
		*prevToken == *p.nextToken {
		p.nextToken = nil
	}

	return result, nil
}

func newServiceMetadataMiddleware_opGetTagValues(region string) *awsmiddleware.RegisterServiceMetadata {
	return &awsmiddleware.RegisterServiceMetadata{
		Region:        region,
		ServiceID:     ServiceID,
		SigningName:   "tagging",
		OperationName: "GetTagValues",
	}
}
