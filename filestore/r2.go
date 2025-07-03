package filestore

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type r2FileStore struct {
	client *s3.Client
	bucket string
}

func NewR2FileStore(accountId, accessKeyId, secretAccessKey, region, bucket string) (FileStore, error) {
	r2Resolver := aws.EndpointResolverFunc(
		func(service, region string) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL: fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountId),
			}, nil
		},
	)

	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithEndpointResolver(r2Resolver),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKeyId, secretAccessKey, ""),
		),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("fail to load r2filestore config: %w", err)
	}

	client := s3.NewFromConfig(cfg)

	return &r2FileStore{
		client: client,
		bucket: bucket,
	}, nil
}

func (s *r2FileStore) UploadFile(ctx context.Context, key string, reader io.Reader) error {
	obj := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   reader,
	}
	_, err := s.client.PutObject(ctx, obj)
	if err != nil {
		return fmt.Errorf("fail to upload to r2: %w", err)
	}

	return nil
}
