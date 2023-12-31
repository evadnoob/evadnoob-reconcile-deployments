package packages

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	cryptossh "golang.org/x/crypto/ssh"

	"slack-reconcile-deployments/internal/manifest"
	"slack-reconcile-deployments/internal/ssh"
)

// Packages manage installed packages on remote system over ssh.
//
// This implementation assumes debian/ubuntu, uses apt-get. An improvement
// could be to make an abstraction for packages and package managers for different
// systems, like rpm, yum, pacman, etc.
type Packages struct {
	// log is the logger
	log *zap.SugaredLogger
	// ssh client to invoke package commands on
	ssh *ssh.Client
	// manifest is the manifest for the host
	manifest *manifest.Manifest
	// packages is a list of all packages and their status on the target system
	packages map[string]manifest.Package
}

// NewPackages creates a new packages management object
func NewPackages(log *zap.SugaredLogger, m *manifest.Manifest, ssh *ssh.Client) *Packages {
	return &Packages{
		log:      log,
		ssh:      ssh,
		manifest: m,
		packages: make(map[string]manifest.Package),
	}
}

// Update update package repository
func (p *Packages) Update() error {
	out, err := p.ssh.Execf(`DEBIAN_FRONTEND=noninteractive apt-get update`)
	if err != nil {
		return errors.Wrap(err, "error on apt-get update")
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		text := scanner.Text()
		if len(strings.TrimSpace(text)) > 0 {
			p.log.Infof("ssh> %s", text)
		}
	}
	if err := scanner.Err(); err != nil {
		p.log.Warn("reading stdout:", err)
	}
	return nil
}

// Query queries packages from container
// Use dpkg-query to list installed packages.
// This implementation assumes debian/ubuntu
func (p *Packages) Query() (map[string]manifest.Package, error) {
	if len(p.manifest.Packages) == 0 {
		return nil, errors.New("no packages provided to query")
	}
	// generate a list of package names to query
	names := make([]string, 0, len(p.manifest.Packages))
	for i := range p.manifest.Packages {
		names = append(names, p.manifest.Packages[i].Name)
	}

	// example:
	//  dpkg-query -W -f='${binary:Package},${Version},${db:Status-Status}\n'| head
	//  adduser,3.118,installed
	//  apt,2.2.4,installed
	//  base-files,11.1+deb11u7,installed
	//  base-passwd,3.5.51,installed
	//
	// Arguments for the dpkg-query are:d
	// -W show, just like --list but allows formatted output
	// -f format output
	//
	// package states:
	// installed: ok, package is installed
	// config-files: only config-files are installed, but binary is not, we'll treat this as not-installed
	// half-configured: likely not good, re-install also to fix.
	// ${db:Status:Status} is the short state, like 'installed'
	//
	// exit status:
	// dpkg-query returns 1 if packages are not installed
	// dpkg-query returns 2 if there is an error
	out, err := p.ssh.Execf(`DEBIAN_FRONTEND=noninteractive /usr/bin/dpkg-query -W `+
		`'-f=${binary:Package},${Version},${db:Status-Status}\n' %s`, strings.Join(names, " "))
	if err != nil {
		var exitErr *cryptossh.ExitError
		if errors.As(err, &exitErr) {
			p.log.Infof("error on dpkg-query: exit error %v, exit status: %v", err, exitErr.ExitStatus())
			if exitErr.ExitStatus() == 1 {
				// dpkg-query returns 1 if packages are not installed, this is ok, we'll re-install
				p.log.Infof("dpkg-query returned 1, packages not installed, this is ok, %v", err)
			} else {
				return nil, errors.Wrap(err, "error on dpkg-query")
			}
		}
	}

	pkglist := make(map[string]manifest.Package)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		text := scanner.Text()
		p.log.Infof("ssh> %s", text)
		parts := strings.Split(text, ",")
		pkg := manifest.Package{
			Name:    parts[0],
			Version: parts[1],
			Status:  parts[2],
		}
		pkglist[pkg.Name] = pkg
	}

	if err := scanner.Err(); err != nil {
		p.log.Warn("reading stdout:", err)
	}

	return pkglist, nil
}

// Install installs provided packages
func (p *Packages) Install(pkgs ...manifest.Package) error {
	if len(pkgs) == 0 {
		return errors.New("no packages provided to install")
	}

	names := make([]string, 0, len(pkgs))
	for i := range pkgs {
		names = append(names, pkgs[i].Name)
	}

	// invoke the fixInvokeRcd to fix invoke-rc.d: could not determine current runlevel
	// docker containers prevent service restart by default, this is the work-around
	if p.manifest.Provider == manifest.ProviderBackendDocker {
		if err := p.fixInvokeRcd(); err != nil {
			return errors.Wrap(err, "error fixing invoke-rc.d")
		}
	}

	out, err := p.ssh.Execf(`DEBIAN_FRONTEND=noninteractive apt-get install -y ` + strings.Join(names, " "))
	if err != nil {
		return errors.Wrap(err, "error on apt-get install")
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		text := scanner.Text()
		p.log.Infof("ssh> %s", text)
	}
	if err := scanner.Err(); err != nil {
		p.log.Warn("reading stdout:", err)
	}

	return nil
}

// Remove removes provided packages
func (p *Packages) Remove(purge bool, pkgs ...manifest.Package) error {
	if len(pkgs) == 0 {
		return errors.New("no packages provided to remove")
	}
	names := make([]string, 0, len(pkgs))
	for i := range pkgs {
		names = append(names, pkgs[i].Name)
	}

	p.log.Infof("removing packages (purge? %v), %s", purge, strings.Join(names, ","))

	// assume run level is fixed already
	// // invoke the fixInvokeRcd to fix invoke-rc.d: could not determine current runlevel
	// // docker containers prevent service restart by default, this is the work-around
	// if p.manifest.Provider == manifest.ProviderBackendDocker {
	//	 if err := p.fixInvokeRcd(); err != nil {
	//		return errors.Wrap(err, "error fixing invoke-rc.d")
	// 	 }
	// }

	purgeOrRemove := "remove"
	if purge {
		purgeOrRemove = "purge"
	}
	out, err := p.ssh.Execf(`DEBIAN_FRONTEND=noninteractive apt-get %s -y `+strings.Join(names, " "),
		purgeOrRemove)
	if err != nil {
		return errors.Wrap(err, "error on apt-get remove")
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		text := scanner.Text()
		p.log.Infof("ssh> %s", text)
	}
	if err := scanner.Err(); err != nil {
		p.log.Warn("reading stdout:", err)
	}

	return nil
}

// fixInvokeRcd fixes invoke-rc.d in docker
//
// The errors you will see are:
// invoke-rc.d: could not determine current runlevel
// invoke-rc.d: policy-rc.d denied execution of restart.
//
// fixInvokeRcd needs to be run before apt-get install because
// the services will be started as part install, and they generate
// errors.
func (p *Packages) fixInvokeRcd() error {
	out, err := p.ssh.Execf(`printf '#!/bin/sh\nexit 0\n' > /usr/sbin/policy-rc.d`)
	if err != nil {
		return errors.Wrap(err, "error on service start")
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		text := scanner.Text()
		p.log.Infof("ssh> %s", text)
	}
	if err := scanner.Err(); err != nil {
		p.log.Warn("reading stdout:", err)
	}

	return nil
}

// RestartService starts a service that was installed.
func (p *Packages) RestartService(name string) error {
	//  Use --full-restart to stop the service and restart it
	out, err := p.ssh.Execf(fmt.Sprintf(`DEBIAN_FRONTEND=noninteractive service %s restart`, name))
	if err != nil {
		return errors.Wrap(err, "error on service start")
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		text := scanner.Text()
		p.log.Infof("ssh> %s", text)
	}
	if err := scanner.Err(); err != nil {
		p.log.Warn("reading stdout:", err)
	}

	return nil
}
