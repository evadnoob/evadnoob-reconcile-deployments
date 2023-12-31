package files

import (
	"fmt"
	"testing"

	"gotest.tools/v3/assert"

	"slack-reconcile-deployments/internal/manifest"
	"slack-reconcile-deployments/internal/ssh"
	"slack-reconcile-deployments/internal/testhelpers"
)

func testRemoteFilesStat(t *testing.T, f *testhelpers.DockerTestFixtures) {
	sshClient, err := ssh.New(f.Log, true, f.PrivateKey,
		"root", "", fmt.Sprintf("%s:%d", "127.0.0.1", f.SSHPort))
	assert.NilError(t, err, "ssh client")

	fm := New(f.Log, &manifest.Manifest{}, sshClient)
	stat, err := fm.Stat(&manifest.File{
		Path: "/etc/os-release",
	})
	assert.NilError(t, err, "stat /etc/os-release")
	assert.Check(t, stat.Sha256 != "", "stat sha256")
	assert.Check(t, len(stat.Sha256) == 64, "stat len(sha256)")
	assert.Check(t, stat.Size > 0, "stat size")
	assert.Check(t, stat.Owner != "", "stat owner")
	assert.Check(t, stat.Group != "", "stat group")
	assert.Check(t, stat.LastModifiedTime != "", "stat last modified time")
	assert.Check(t, stat.Type != "", "stat type")

}
