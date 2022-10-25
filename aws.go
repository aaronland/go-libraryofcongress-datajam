package datajam

import (
	"fmt"
)

var AWS_S3_BUCKET string
var AWS_S3_URI string
var AWS_S3_REGION string

func init() {
	AWS_S3_BUCKET = ""
	AWS_S3_REGION = ""
	AWS_S3_URI = fmt.Sprintf("https://%s.s3-%s.amazonaws.com/", AWS_S3_BUCKET, AWS_S3_REGION)
}
