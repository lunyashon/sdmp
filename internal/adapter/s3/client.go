package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/port"
)

var _ port.S3 = (*Client)(nil)

type Config struct {
	AccessKey string
	SecretKey string
	Endpoint  string
	Region    string
	Bucket    string
}

type Client struct {
	api      *awss3.Client
	uploader *manager.Uploader
	bucket   string
	log      *slog.Logger
}

func New(ctx context.Context, cfg Config, log *slog.Logger) (*Client, error) {
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("s3 credentials are empty")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 bucket is empty")
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = "https://storage.yandexcloud.net"
	}
	if cfg.Region == "" {
		cfg.Region = "ru-central1"
	}
	if log == nil {
		log = slog.Default()
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey,
			cfg.SecretKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	api := awss3.NewFromConfig(awsCfg, func(o *awss3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true
	})

	return &Client{
		api:      api,
		uploader: manager.NewUploader(api),
		bucket:   cfg.Bucket,
		log:      log,
	}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.api.HeadBucket(ctx, &awss3.HeadBucketInput{
		Bucket: aws.String(c.bucket),
	})
	if err != nil {
		return fmt.Errorf("head bucket %s: %w", c.bucket, err)
	}
	return nil
}

func (c *Client) Put(ctx context.Context, key string, body io.Reader, opts port.PutOptions) error {
	key, err := normalizeKey(key)
	if err != nil {
		return err
	}

	input := &awss3.PutObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
		Body:   body,
	}
	if opts.ContentType != "" {
		input.ContentType = aws.String(opts.ContentType)
	}

	if _, err := c.uploader.Upload(ctx, input); err != nil {
		return fmt.Errorf("put object %s: %w", key, err)
	}

	c.log.InfoContext(ctx, "s3 put", "bucket", c.bucket, "key", key)
	return nil
}

func (c *Client) Get(ctx context.Context, key string) (*port.Object, error) {
	key, err := normalizeKey(key)
	if err != nil {
		return nil, err
	}

	out, err := c.api.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("%w: %s", domain.ErrNotFound, key)
		}
		return nil, fmt.Errorf("get object %s: %w", key, err)
	}

	obj := &port.Object{
		Key:  key,
		Body: out.Body,
	}
	if out.ContentType != nil {
		obj.ContentType = *out.ContentType
	}
	if out.ContentLength != nil {
		obj.ContentLength = *out.ContentLength
	}
	return obj, nil
}

func (c *Client) Delete(ctx context.Context, key string) error {
	key, err := normalizeKey(key)
	if err != nil {
		return err
	}

	_, err = c.api.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete object %s: %w", key, err)
	}
	return nil
}

func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	key, err := normalizeKey(key)
	if err != nil {
		return false, err
	}

	_, err = c.api.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("head object %s: %w", key, err)
	}
	return true, nil
}

func (c *Client) List(ctx context.Context, prefix string) ([]port.ObjectInfo, error) {
	prefix = strings.TrimPrefix(strings.ReplaceAll(prefix, "\\", "/"), "/")
	if strings.Contains(prefix, "..") {
		return nil, fmt.Errorf("%w: invalid prefix", domain.ErrInvalidInput)
	}

	var (
		out   []port.ObjectInfo
		token *string
	)
	for {
		resp, err := c.api.ListObjectsV2(ctx, &awss3.ListObjectsV2Input{
			Bucket:            aws.String(c.bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: token,
		})
		if err != nil {
			return nil, fmt.Errorf("list prefix %s: %w", prefix, err)
		}
		for _, obj := range resp.Contents {
			item := port.ObjectInfo{
				Key:  aws.ToString(obj.Key),
				Size: aws.ToInt64(obj.Size),
			}
			if obj.LastModified != nil {
				item.LastModified = *obj.LastModified
			}
			out = append(out, item)
		}
		if !aws.ToBool(resp.IsTruncated) {
			break
		}
		token = resp.NextContinuationToken
	}
	if out == nil {
		out = []port.ObjectInfo{}
	}
	return out, nil
}

func normalizeKey(key string) (string, error) {
	key = strings.TrimSpace(strings.ReplaceAll(key, "\\", "/"))
	key = strings.TrimPrefix(key, "/")
	if key == "" || strings.Contains(key, "..") {
		return "", fmt.Errorf("%w: invalid object key", domain.ErrInvalidInput)
	}
	return key, nil
}

func isNotFound(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound":
			return true
		}
	}
	return false
}
