package manifest

import (
	"strings"
	"testing"

	"golang.org/x/exp/slices"

	"gotest.tools/v3/assert"
)

// TestHostManifestParsingForDocker test manifest for docker
func TestHostManifestForDocker(t *testing.T) {
	m, err := NewFromFile("testdata/manifest_docker.yaml",
		"testdata/packages.yaml")
	assert.NilError(t, err)
	assert.Check(t, m != nil, "manifest should not be nil")
	assert.Equal(t, m.Provider, ProviderBackendDocker)
	assert.Check(t, len(m.Packages) == 4, "expected 4 packages")
	for _, pkgs := range m.Packages {
		// expect services to have some files to install
		if pkgs.Kind == PackageKindService {
			assert.Check(t, len(pkgs.Files) > 0, "expected files")
		}
		for _, f := range pkgs.Files {
			assert.Check(t, f.Mode != "")
			assert.Check(t, f.Path != "")
			assert.Check(t, f.Content != "")
		}
	}

	// find the php fpm package in the packages
	indexOfPhp := slices.IndexFunc(m.Packages, func(p Package) bool {
		return strings.HasPrefix(p.Name, "php") && strings.HasSuffix(p.Name, "-fpm")
	})
	phpFpm, err := m.FindPackage("php", "-fpm")
	assert.NilError(t, err, "find php fpm package")
	assert.Check(t, indexOfPhp != -1, "packages find php fpm")
	assert.Equal(t, phpFpm.Version, "8.2")

}

// TestHostManifestForAWS test manifest for ec2
func TestHostManifestForAWS(t *testing.T) {
	m, err := NewFromFile("testdata/manifest_ec2.yaml",
		"testdata/packages.yaml")
	assert.NilError(t, err)
	assert.Check(t, m != nil, "manifest should not be nil")
	assert.Equal(t, m.Provider, ProviderBackendEC2)
	assert.Check(t, len(m.Packages) == 4, "expected 4 packages")
	for _, pkgs := range m.Packages {
		// expect services to have some files to install
		if pkgs.Kind == PackageKindService {
			assert.Check(t, len(pkgs.Files) > 0, "expected files")
		}
		for _, f := range pkgs.Files {
			assert.Check(t, f.Mode != "")
			assert.Check(t, f.Path != "")
			assert.Check(t, f.Content != "")
		}
	}

	// test parameters exist and are populated
	tests := []struct {
		key      string
		expected string
	}{
		{key: "image-id", expected: "test-image-id"},
		{key: "size", expected: "t4g.nano"},
		{key: "key-name", expected: "test-key-name"},
		{key: "security-group-id", expected: "sg-test"},
		{key: "subnet-id", expected: "test-subnet-id"},
	}
	for _, test := range tests {
		v, ok := m.Parameters[test.key]
		assert.Check(t, ok)
		assert.Equal(t, v, test.expected)
	}
}

func TestBadFileMode(t *testing.T) {
	_, err := NewFromFile("testdata/manifest_docker.yaml",
		"testdata/packages_bad_file_modes.yaml")
	assert.ErrorContains(t, err, "invalid file mode")

}
