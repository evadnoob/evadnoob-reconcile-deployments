package files

import (
	"testing"

	"slack-reconcile-deployments/internal/testhelpers"
)

// TestFilesTestSuite sets up the testing suite after starting a single
// docker shared amongst files tests.
func TestFilesTestSuite(t *testing.T) {
	s := testhelpers.NewDockerTestFixtures(t)
	s.Run(t)

	t.Run("test remote file stat", func(t *testing.T) {
		testRemoteFilesStat(t, s)
	})
	t.Run("test files render", func(t *testing.T) {
		testFileRender(t, s)
	})
	t.Run("test files remove", func(t *testing.T) {
		testFileRemove(t, s)
	})
}
