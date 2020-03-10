package s3

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDefaultSession(t *testing.T) {
	_, err := DefaultSession()
	if err != nil {
		t.Errorf("Failed to get an AWS Session")
	}
}

func TestS3Url(t *testing.T) {
	inputs := []struct {
		url    string
		result bool
		bucket string
		region string
		key    string
	}{
		{
			"https://www.nikogura.com",
			false,
			"",
			"",
			"",
		},
		{
			"https://dbt-tools.s3.us-east-1.amazonaws.com/catalog/1.2.3/darwin/amd64/catalog",
			true,
			"dbt-tools",
			"us-east-1",
			"catalog/1.2.3/darwin/amd64/catalog",
		},
	}

	for _, tc := range inputs {
		t.Run(tc.url, func(t *testing.T) {
			fmt.Printf("Testing %s\n", tc.url)
			ok, meta := S3Url(tc.url)

			assert.True(t, ok == tc.result, fmt.Sprintf("%s does not meet expectations", tc.url))
			assert.True(t, tc.bucket == meta.Bucket, fmt.Sprintf("Bucket %q doesn't look right", meta.Bucket))
			assert.True(t, tc.region == meta.Region, fmt.Sprintf("Region %q doesn't look right.", meta.Region))
			assert.True(t, tc.key == meta.Key, fmt.Sprintf("Key %q doesn't look right.", meta.Key))
		})
	}
}
