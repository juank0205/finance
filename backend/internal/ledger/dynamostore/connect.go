package dynamostore

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// Connect returns a DynamoDB client. In Lambda the region and credentials come from the environment.
// A non-empty endpoint means DynamoDB Local: it gets fixed dummy credentials, so real ones are never
// sent to it and a local run can never reach the AWS account.
func Connect(ctx context.Context, endpoint string) (*dynamodb.Client, error) {
	var opts []func(*config.LoadOptions) error
	if endpoint != "" {
		opts = append(opts,
			config.WithRegion("eu-west-3"),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("local", "local", "")),
		)
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			// DynamoDB Local sends wrong CRC32 checksums on some error responses.
			o.DisableValidateResponseChecksum = true
		}
	}), nil
}
