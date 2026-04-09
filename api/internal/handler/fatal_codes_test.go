// fatal_codes_test.go locks the fatalErrorCodes allowlist so a new
// validator code can't be added without an explicit decision about
// whether it blocks publish or not.
//
// Caught a real Sprint 4 PR3 regression: first_comment_unsupported
// was added to validate.go but missed from this allowlist, so the
// strict-reject contract for Bluesky/Threads silently failed and
// the publish loop went ahead and posted the parent anyway.

package handler

import (
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/platform"
)

// TestFatalErrorCodes_Sprint4 locks the explicit Sprint 4 PR3 codes
// in the fatal allowlist. The matching test in Sprint 2 / 3 would
// have caught the regression at PR3 review time if it existed.
func TestFatalErrorCodes_Sprint4(t *testing.T) {
	required := []string{
		platform.CodeFirstCommentUnsupported,
		platform.CodeFirstCommentTooLong,
	}
	for _, code := range required {
		if !fatalErrorCodes[code] {
			t.Errorf("validator code %q must be in fatalErrorCodes — see Sprint 4 PR3 retro", code)
		}
	}
}

// TestFatalErrorCodes_Threads locks the Sprint 2 thread codes that
// were also missing before today's audit.
func TestFatalErrorCodes_Threads(t *testing.T) {
	required := []string{
		platform.CodeThreadsUnsupported,
		platform.CodeThreadPositionsNotContiguous,
		platform.CodeThreadMixedWithSingle,
	}
	for _, code := range required {
		if !fatalErrorCodes[code] {
			t.Errorf("thread code %q must be in fatalErrorCodes", code)
		}
	}
}

// TestFatalErrorCodes_Media locks the Sprint 2 media library codes.
func TestFatalErrorCodes_Media(t *testing.T) {
	required := []string{
		platform.CodeMediaIDNotFound,
		platform.CodeMediaIDNotInProject,
		platform.CodeMediaNotUploaded,
	}
	for _, code := range required {
		if !fatalErrorCodes[code] {
			t.Errorf("media code %q must be in fatalErrorCodes", code)
		}
	}
}
