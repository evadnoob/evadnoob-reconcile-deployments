package files

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"slack-reconcile-deployments/internal/logging"
	"slack-reconcile-deployments/internal/manifest"
)

// TestFileManager_Render tests rendering files from templates, does not
// test transfer over ssh, that scenario is covered in the docker tests.
func TestFileManager_Render(t *testing.T) {
	m := &manifest.Manifest{
		Packages: []manifest.Package{
			{Name: "nginx", Version: "latest", Kind: manifest.PackageKindService,
				Parameters: map[string]string{
					"PhpFpmVersion": "8.2",
				},
				Files: []manifest.File{
					{Path: "/etc/nginx/sites-available/default",
						Content: "embed://templates/etc_nginx_sites_available_default"},
				},
			},
			{Name: "php8.2-fpm", Version: "8.2", Kind: manifest.PackageKindService,
				Files: []manifest.File{
					{Path: "/var/www/html/info.php",
						Content: "embed://templates/var_www_html_index_php"},
				},
			},
		},
	}
	fm := New(logging.New(t.Name(), false), m, nil)

	lastModifiedDate := time.Now().UTC().Format(time.RFC3339)
	for _, pkg := range fm.manifest.Packages {
		for _, f := range pkg.Files {
			assert.Check(t, f.Path != "", "error: file path not set")
			reader, err := fm.Render(&pkg, &f, map[string]string{
				FileTemplateKeyLastModifiedDate: lastModifiedDate,
			})
			assert.NilError(t, err, "render %s", f.Path)
			assert.Check(t, reader != nil, "reader is nil %s", f.Path)
			all, err := io.ReadAll(reader)
			assert.NilError(t, err, "read all %s", f.Path)
			assert.Check(t, len(all) > 0, "read len(all)  %s", f.Path)
			assert.Check(t, string(all) != "", "read string(all)  %s", f.Path)
			assert.Check(t, strings.Contains(string(all),
				fmt.Sprintf("generated file %s", lastModifiedDate)),
				"contains generated date %s", f.Path)
			if f.Path == "/etc/nginx/sites-available/default" {
				assert.Check(t, strings.Contains(string(all),
					`fastcgi_pass unix:/run/php/php8.2-fpm.sock`),
					"contains php8.2-fpm")
			}
		}
	}
}
