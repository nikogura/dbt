package s3

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"log"
	"os"
	"regexp"
)

const AWS_ID_ENV_VAR = "AWS_ACCESS_KEY_ID"
const AWS_SECRET_ENV_VAR = "AWS_SECRET_ACCESS_KEY"
const AWS_REGION_ENV_VAR = "AWS_DEFAULT_REGION"

// DefaultSession creates a default AWS session from local config path.  Hooks directly into credentials if present, or Credentials Provider if configured.
func DefaultSession() (awssession *session.Session, err error) {
	if os.Getenv(AWS_ID_ENV_VAR) == "" && os.Getenv(AWS_SECRET_ENV_VAR) == "" {
		_ = os.Setenv("AWS_SDK_LOAD_CONFIG", "true")
	}

	awssession, err = session.NewSession()
	if err != nil {
		log.Fatalf("Failed to create aws session")
	}

	// For some reason this doesn't get picked up automatically, but we'll set it if it's present in the enviornment.
	if os.Getenv(AWS_REGION_ENV_VAR) != "" {
		awssession.Config.Region = aws.String(os.Getenv(AWS_REGION_ENV_VAR))
	}

	return awssession, err
}

// S3Meta a struct for holding metadata for S3 Objects.  There's probably already a struct that holds this, but this is all I need.
type S3Meta struct {
	Bucket string
	Region string
	Key    string
	Url    string
}

// S3Url returns true, and a metadata struct if the url given appears to be in s3
func S3Url(url string) (ok bool, meta S3Meta) {
	// Check to see if it's an s3 URL.
	s3Url := regexp.MustCompile(`https?://(.*)\.s3\.(.*)\.amazonaws.com/(.*)`)

	fmt.Printf("testing %s\n", url)
	matches := s3Url.FindAllStringSubmatch(url, -1)

	if len(matches) == 0 {
		return ok, meta
	}

	match := matches[0]

	if len(match) == 4 {
		meta = S3Meta{
			Bucket: match[1],
			Region: match[2],
			Key:    match[3],
			Url:    url,
		}

		ok = true
		return ok, meta
	}

	return ok, meta
}
