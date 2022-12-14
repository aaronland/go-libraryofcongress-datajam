package datajam

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"gocloud.dev/blob"
	"gocloud.dev/blob/s3blob"
	_ "log"
	"net/url"
)

const IS_LIBRARYOFCONGRESS_S3 string = "github.com/aaronland/go-libraryofcongress-datajam#is_libraryofcongress_s3"

func OpenBucket(ctx context.Context, uri string) (context.Context, *blob.Bucket, error) {

	u, err := url.Parse(uri)

	if err != nil {
		return nil, nil, err
	}

	is_libraryofcongress_s3 := false

	switch u.Scheme {
	case "s3":

		if u.Host == AWS_S3_BUCKET {
			is_libraryofcongress_s3 = true
		}

	case "loc", "libraryofcongress":

		uri = fmt.Sprintf("s3://%s?region=%s", AWS_S3_BUCKET, AWS_S3_REGION)
		is_libraryofcongress_s3 = true

	default:
		// pass
	}

	var bucket *blob.Bucket

	if is_libraryofcongress_s3 {

		sess, err := session.NewSession(&aws.Config{
			Region:      aws.String(AWS_S3_REGION),
			Credentials: credentials.AnonymousCredentials,
		})

		if err != nil {
			return nil, nil, fmt.Errorf("Failed to create AWS session, %w", err)
		}

		// SKIPMETADATA GOES HERE

		b, err := s3blob.OpenBucket(ctx, sess, AWS_S3_BUCKET, nil)

		if err != nil {
			return nil, nil, fmt.Errorf("Failed to open bucket, %w", err)
		}

		bucket = b

	} else {

		b, err := blob.OpenBucket(ctx, uri)

		if err != nil {
			return nil, nil, fmt.Errorf("Failed to open bucket, %w", err)
		}

		bucket = b
	}

	ctx = context.WithValue(ctx, IS_LIBRARYOFCONGRESS_S3, is_libraryofcongress_s3)
	return ctx, bucket, nil
}
