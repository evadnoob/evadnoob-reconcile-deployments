package ssh

import (
	"bytes"
	"fmt"
	"testing"

	"gotest.tools/v3/assert"

	"slack-reconcile-deployments/internal/testhelpers"
)

func testDockerSSH(t *testing.T, f *testhelpers.DockerTestFixtures) {
	// using localhost:port here for now, the docker container is exporting and rebinding ports for ssh/22
	client, err := New(f.Log, true, f.PrivateKey,
		"root", "", fmt.Sprintf("%s:%d", "127.0.0.1", f.SSHPort))
	if err != nil {
		fmt.Printf("error creating ssh client %+v", err)
	}
	assert.NilError(t, err, "run ssh")

	out, err := client.Execf("/usr/bin/whoami")
	assert.NilError(t, err, "exec whoami")
	assert.Equal(t, string(out), "root\n", "whoami")
}

func testSecureCopyClient(t *testing.T, f *testhelpers.DockerTestFixtures) {
	client, err := New(f.Log, true, f.PrivateKey,
		"root", "", fmt.Sprintf("%s:%d", "127.0.0.1", f.SSHPort))
	if err != nil {
		fmt.Printf("error creating ssh client %+v", err)
	}

	scp := NewSecureCopyClient(f.Log, client)
	contents := []byte("hello this is a test")
	err = scp.Copy(bytes.NewReader(contents), "/tmp/hello.txt")
	assert.NilError(t, err, "copy file to target")

	filename := "/tmp/hello.txt"
	out, err := client.Execf(fmt.Sprintf("/bin/cat %s", filename))
	assert.NilError(t, err, "exec cat %s", filename)
	assert.Check(t, out != nil, "nil out")
	assert.Check(t, bytes.Equal(out, contents), "cat %s", filename)
}

// TestSecureShellSuite sets up the testing suite after starting a single
// docker shared amongst tests. This could be a problem if a test needs
// a clean container, will come back for this if needed in the future.
func TestSecureShellSuite(t *testing.T) {
	s := testhelpers.NewDockerTestFixtures(t)
	s.Run(t)

	t.Run("test private key public key", func(t *testing.T) {
		testDockerSSH(t, s)
	})
	t.Run("test scp", func(t *testing.T) {
		testSecureCopyClient(t, s)
	})
}
