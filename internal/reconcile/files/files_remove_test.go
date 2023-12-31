package files

import (
	"fmt"
	"testing"

	"gotest.tools/v3/assert"

	"slack-reconcile-deployments/internal/manifest"
	"slack-reconcile-deployments/internal/ssh"
	"slack-reconcile-deployments/internal/testhelpers"
)

func testFileRemove(t *testing.T, f *testhelpers.DockerTestFixtures) {
	sshClient, err := ssh.New(f.Log, true, f.PrivateKey,
		"root", "", fmt.Sprintf("%s:%d", "127.0.0.1", f.SSHPort))
	assert.NilError(t, err, "ssh client")

	fm := New(f.Log, &manifest.Manifest{}, sshClient)
	pkg := manifest.Package{
		Files: []manifest.File{{
			Path:    "/var/www/html/index.php",
			Content: "embed://templates/var_www_html_index_php",
		}},
	}

	_, err = sshClient.Execf("mkdir -p /var/www/html && echo 'hello world' > /var/www/html/index.php")
	assert.NilError(t, err, "create /var/www/html/index.php")

	err = fm.Remove(pkg)
	assert.NilError(t, err, "remove")

	stat, err := fm.Stat(&pkg.Files[0])
	assert.Check(t, stat == nil, "stat nil")
	assert.ErrorContains(t, err, "file does not exist")
}
