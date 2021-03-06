package s3copy

import (
	"fmt"
	"github.com/evergreen-ci/evergreen/plugin"
	"github.com/evergreen-ci/evergreen/util"
	"github.com/goamz/goamz/s3"
	"regexp"
	"strings"
)

var (
	BucketNameRegex = regexp.MustCompile("^[a-z0-9\\-.]+$")
)

func validateS3BucketName(bucket string) error {
	// if it's an expandable string, we can't expand yet since we don't have
	// access to the task config expansions. So, we defer till during runtime
	// to do the validation
	if plugin.IsExpandable(bucket) {
		return nil
	}
	if len(bucket) < 3 {
		return fmt.Errorf("must be at least 3 characters")
	}
	if len(bucket) > 63 {
		return fmt.Errorf("must be no more than 63 characters")
	}
	if strings.HasPrefix(bucket, ".") || strings.HasPrefix(bucket, "-") {
		return fmt.Errorf("must not begin with a period or hyphen")
	}
	if strings.HasSuffix(bucket, ".") || strings.HasSuffix(bucket, "-") {
		return fmt.Errorf("must not end with a period or hyphen")
	}
	if strings.Contains(bucket, "..") {
		return fmt.Errorf("must not have two consecutive periods")
	}
	/*
		if !BucketNameRegex.MatchString(bucket) {
			return fmt.Errorf("must contain only lowercase letters, numbers," +
				" hyphens, and periods")
		}
	*/
	return nil
}

func validS3Permissions(perm string) bool {
	return util.SliceContains(
		[]s3.ACL{
			s3.Private,
			s3.PublicRead,
			s3.PublicReadWrite,
			s3.AuthenticatedRead,
			s3.BucketOwnerRead,
			s3.BucketOwnerFull,
		},
		s3.ACL(perm),
	)
}
