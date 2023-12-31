package manifest

import (
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

// ProviderBackend is a backend where ssh will be running.
type ProviderBackend string

const (
	// ProviderBackendDocker ProviderBackendDocker is the docker provider backend
	ProviderBackendDocker = ProviderBackend("docker")
	// ProviderBackendEC2 is the ec2 provider backend
	ProviderBackendEC2 = ProviderBackend("ec2")
	// ProviderBackendSlack ProviderBackendSlack is the slack host backend provider
	ProviderBackendSlack = ProviderBackend("slack")
	// ProviderBackendLinode ProviderBackendLinode is the linode host backend provider
	ProviderBackendLinode = ProviderBackend("linode")
)

// Manifest is a manifest describing desired state of deployment A manifest can
// be compared to a providers inventory to perform a reconcile operation.
//
// Both Packages and Files are constructed from different files, so use -,omitempty to ignore them here.
type Manifest struct {
	// ID is a unique id for the host. Host will be tagged/named with this id
	// and used to determine if the host exists
	ID string `yaml:"id"`
	// Provider is a backend to use. The docker provider backend is used for testing.
	Provider ProviderBackend `yaml:"provider"`
	// Packages are the desired packages to be on the target
	Packages []Package `yaml:"-"`
	// Parameters is a map of parameters to be used when creating this host.
	// Using a simple mapping here to allow different set of parameters based
	// on provider. Parameters are optional, when using docker we don't provide
	// any parameters, yet.
	Parameters map[string]string `yaml:"parameters,omitempty"`
}

// PackageKind is the kind package: binary or service
type PackageKind string

const (
	// PackageKindService a service is treated differently than a binary during
	// package installation
	PackageKindService = "service"
	// PackageKindBinary is binary, not treated like a service
	PackageKindBinary = "binary"
)

// Package is a package to be installed on a host during reconcile
type Package struct {
	// Name is the name of the package
	Name string `yaml:"name"`
	// Version is the package version, very simple, no version >= support etc.
	Version string `yaml:"version"`
	// Kind is either binary or service
	Kind PackageKind `yaml:"kind"`
	// Status is "installed" or "Not-installed" to be used only at runtime
	Status string `yaml:"-"`
	// Files are files to transfer to the target  host
	Files []File `yaml:"files"`
	// Parameters is a map of parameters to be used when rendering this package.
	Parameters map[string]string `yaml:"parameters,omitempty"`
}

// File is a template that will be rendered and copied to a target host
type File struct {
	// Path is the target path on the host
	Path string `yaml:"path"`
	// Mode is the file mode in octal:
	// Example:
	// 	0644 -rw-r--r--
	//  0777 -rwxrwxrwx
	Mode string `yaml:"mode"`
	// Owner is the owner of the file
	Owner string `yaml:"owner"`
	// Content is the content of the file to be rendered
	Content string `yaml:"content"`
}

// NewFromBytes creates a new manifest from bytes
// Useful from NewFromFile or in tests with arbitrary manifest bytes.
func NewFromBytes(host, packages []byte) (*Manifest, error) {
	if len(host) == 0 {
		return nil, errors.New("cannot create a manifest from an empty byte array")
	}
	if len(packages) == 0 {
		return nil, errors.New("cannot create packages from an empty byte array")
	}
	var m Manifest
	if err := yaml.Unmarshal(host, &m); err != nil {
		return nil, errors.Wrap(err, "error unmarshalling bytes for manifest")
	}

	var pkgs []Package
	if err := yaml.Unmarshal(packages, &pkgs); err != nil {
		return nil, errors.Wrap(err, "error unmarshalling bytes for packages")
	}
	m.Packages = pkgs

	fileModeRE := regexp.MustCompile(`^[0-7]{3,4}$`)
	// validate metadata about file ownership
	for _, pkg := range m.Packages {
		for _, f := range pkg.Files {
			if !fileModeRE.MatchString(f.Mode) {
				return nil, errors.Errorf("invalid file mode %s for file %s", f.Mode, f.Path)
			}
		}
	}

	return &m, nil
}

// NewFromFile reads file then calls NewFromBytes() with bytes from file
// Convenience function when you know where a host manifest is located.
func NewFromFile(hosts, packages string) (*Manifest, error) {
	// load bytes for hosts(one per "manifest") into manifest
	f1, err := os.Open(hosts)
	if err != nil {
		return nil, errors.Wrapf(err, "error opening manifest %s", hosts)
	}
	b1, err := io.ReadAll(f1)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading manifest %s", hosts)
	}

	// load bytes for packages into manifest
	f2, err := os.Open(packages)
	if err != nil {
		return nil, errors.Wrapf(err, "error opening packages %s", packages)
	}
	b2, err := io.ReadAll(f2)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading packages %s", packages)
	}

	return NewFromBytes(b1, b2)
}

// FindPackage finds a package by prefix and suffix
func (m *Manifest) FindPackage(prefix, suffix string) (*Package, error) {
	// find the matching package in manifest packages
	index := slices.IndexFunc(m.Packages, func(p Package) bool {
		return strings.HasPrefix(p.Name, prefix) && strings.HasSuffix(p.Name, suffix)
	})
	if index == -1 {
		return nil, errors.Errorf("no package found with prefix: %s and suffix: %s", prefix, suffix)
	}
	return &m.Packages[index], nil
}
