package r2

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"etica_back/internal/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

func Diagnose(err error) string {
	var response *smithyhttp.ResponseError
	if errors.As(err, &response) {
		switch response.HTTPStatusCode() {
		case 0:
			return "no HTTP response: check network, HTTPS_PROXY and endpoint"
		case 403:
			return "access denied (HTTP 403): check R2 S3 credentials and bucket permissions"
		case 404:
			return "bucket not found (HTTP 404): check R2_BUCKET and account endpoint"
		default:
			return fmt.Sprintf("bucket check failed (HTTP %d)", response.HTTPStatusCode())
		}
	}
	return "bucket check failed: check HTTPS endpoint, network and credentials"
}

// New creates the S3-compatible R2 client without uploading or modifying objects.
func New(cfg config.Config) *s3.Client {
	return s3.New(s3.Options{
		Region:       cfg.R2Region,
		BaseEndpoint: aws.String(cfg.R2Endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.R2AccessKeyID, cfg.R2SecretAccessKey, ""),
		HTTPClient:   &http.Client{Timeout: 15 * time.Second},
	})
}

func Check(ctx context.Context, client *s3.Client, bucket string) error {
	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
	return err
}
