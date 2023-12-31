package reconcile

import (
	"context"
	"testing"

	"gotest.tools/v3/assert"

	"slack-reconcile-deployments/internal/logging"
	"slack-reconcile-deployments/internal/manifest"
)

func TestReconcileDocker(t *testing.T) {
	log := logging.New(t.Name(), false)
	m := &manifest.Manifest{
		ID:       "d496f4b1c0",
		Provider: "docker",
		Packages: []manifest.Package{
			{Name: "netcat-traditional", Version: "latest", Kind: manifest.PackageKindBinary},
			{Name: "dnsutils", Version: "latest", Kind: manifest.PackageKindBinary},
			{Name: "nginx", Version: "latest", Kind: manifest.PackageKindService,
				Parameters: map[string]string{
					"PhpFpmVersion": "8.2",
				},
				Files: []manifest.File{
					{Path: "/etc/nginx/sites-available/default", Mode: "0644", Owner: "root:root",
						Content: `embed://templates/etc_nginx_sites_available_default`},
				}},
			{Name: "php8.2-fpm", Version: "8.2", Kind: manifest.PackageKindService,
				Files: []manifest.File{
					{Path: "/var/www/html/info.php", Mode: "0777", Owner: "root:root", Content: `<?php phpinfo();?>`},
				}},
		},
	}
	err := Run(context.TODO(), log, m, Reconcile)
	assert.NilError(t, err)
	log.Infof("reconciler is finished")
}
