package files

import (
	"bytes"
	"fmt"
	"io"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"slack-reconcile-deployments/internal/manifest"
	"slack-reconcile-deployments/internal/ssh"
	"slack-reconcile-deployments/internal/testhelpers"
)

func testFileRender(t *testing.T, f *testhelpers.DockerTestFixtures) {
	sshClient, err := ssh.New(f.Log, true, f.PrivateKey,
		"root", "", fmt.Sprintf("%s:%d", "127.0.0.1", f.SSHPort))
	assert.NilError(t, err, "ssh client")

	tests := []struct {
		name        string
		packageName string
		version     string
		content     string
		wantErr     bool
	}{
		{packageName: "php8.2-fpm", version: "8.2", name: "/var/www/html/index.php",
			content: "embed://templates/var_www_html_index_php", wantErr: false},
		{packageName: "nginx", version: "latest", name: "/etc/nginx/sites-available/default",
			content: "embed://templates/etc_nginx_sites_available_default", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now().UTC().Format(time.RFC3339)
			fm := New(f.Log, &manifest.Manifest{}, sshClient)
			reader, err := fm.Render(&manifest.Package{
				Name:    tt.packageName,
				Version: tt.version,
			}, &manifest.File{
				Path:    tt.name,
				Content: tt.content,
			}, map[string]string{
				"LastModifiedDate": now,
			})

			assert.NilError(t, err, "render")
			assert.Check(t, reader != nil, "reader")
			b, err := io.ReadAll(reader)
			assert.NilError(t, err, "read all")
			assert.Check(t, b != nil, "b")
			assert.Check(t, len(b) > 0, "len(b)")
			assert.Check(t, bytes.Contains(b, []byte(now)), "contains now")
		})
	}
}
