package reconcile

import (
	"context"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"slack-reconcile-deployments/internal/manifest"
	"slack-reconcile-deployments/internal/reconcile/backend"
	dockerbackend "slack-reconcile-deployments/internal/reconcile/backend/docker"
	ec2backend "slack-reconcile-deployments/internal/reconcile/backend/ec2"
	"slack-reconcile-deployments/internal/reconcile/backend/linode"
	slackbackend "slack-reconcile-deployments/internal/reconcile/backend/slack"
	"slack-reconcile-deployments/internal/reconcile/files"
	"slack-reconcile-deployments/internal/reconcile/packages"
	"slack-reconcile-deployments/internal/ssh"
)

// Operation an operation to perform by reconcile
type Operation string

const (
	// Reconcile operation to reconcile
	Reconcile = Operation("reconcile")
	// Remove operation removes things done by reconcile, only removes
	// packages, use Purge to remove configuration
	Remove = Operation("remove")
	// Purge is just like Remove but removes configuration in addition to packages
	Purge = Operation("purge")
)

// Run runs reconcile with given provider and path to manifest
func Run(ctx context.Context, log *zap.SugaredLogger, m *manifest.Manifest,
	op Operation, options ...func(reconciler backend.ProviderBackendReconciler)) error {
	start := time.Now()
	var err error
	var be backend.ProviderBackendReconciler
	switch m.Provider {
	case manifest.ProviderBackendDocker:
		be = dockerbackend.New(log, m)
	case manifest.ProviderBackendEC2:
		be, err = ec2backend.New(log, ctx, m)
		if err != nil {
			return errors.Wrapf(err, "error on provider backend new %s", m.Provider)
		}
	case manifest.ProviderBackendSlack:
		be = slackbackend.New(log, m)
		for _, option := range options {
			option(be)
		}
	case manifest.ProviderBackendLinode:
		be, err = linode.New(log, ctx, m)
		if err != nil {
			return errors.Wrapf(err, "error on provider backend new %s", m.Provider)
		}
		for _, option := range options {
			option(be)
		}
	default:
		return errors.Errorf("unknown provider %s", m.Provider)
	}

	log.Infof("running backend %+v", be)
	sshClient, err := be.Run(ctx)
	if err != nil {
		return errors.Wrap(err, "error on provider backend reconcile")
	}
	defer be.Close()

	out, err := sshClient.Execf("cat /etc/os-release")
	if err != nil {
		log.Info("warning: unable to get /etc/os-release")
	} else {
		log.Infof("os-release: %s", out)
	}

	reconciler := New(log, m, sshClient)

	switch op {
	case Reconcile:
		if err := reconciler.Reconcile(ctx); err != nil {
			return errors.Wrap(err, "error on reconciler")
		}
	case Remove, Purge:
		if err := reconciler.Remove(ctx, false); err != nil {
			return errors.Wrap(err, "error on reconciler")
		}
	default:
		return errors.Errorf("unknown reconcile op: %s", op)
	}

	log.Infof("reconcile done %v", time.Since(start))
	return nil
}

// ProviderReconciler reconciles provider state using packages and a backend(docker or ec2)
type ProviderReconciler struct {
	log      *zap.SugaredLogger
	manifest *manifest.Manifest
	ssh      *ssh.Client
}

// New creates a new provide reconciler
func New(log *zap.SugaredLogger, m *manifest.Manifest, sshClient *ssh.Client) *ProviderReconciler {
	return &ProviderReconciler{
		log:      log,
		manifest: m,
		ssh:      sshClient,
	}
}

// Reconcile run reconcile using backend.
// this function is common to all backends
func (p *ProviderReconciler) Reconcile(_ context.Context) error {
	p.log.Infof("reconcile")

	p.log.Info("spew manifest", spew.Sdump(p.manifest))
	missing := make([]manifest.Package, 0, len(p.manifest.Packages))
	pkgs := packages.NewPackages(p.log, p.manifest, p.ssh)
	pkglist, err := pkgs.Query()
	if err != nil {
		return errors.Wrap(err, "error getting packages from container")
	}
	p.log.Infof("pkgs %d", len(pkglist))

	// diff the desired and actual packages, make a list of packages to install
	for _, pkgDesired := range p.manifest.Packages {
		pkgActual, ok := pkglist[pkgDesired.Name]
		switch {
		case !ok:
			p.log.Infof("desired package %s is missing on target %s",
				pkgDesired.Name, p.manifest.ID)
			missing = append(missing, manifest.Package{Name: pkgDesired.Name, Version: pkgDesired.Version})
		case ok && pkgActual.Status != "installed":
			p.log.Info("desired package %s appears not to be installed, will install status: %s",
				pkgActual.Name, pkgActual.Status)
			missing = append(missing, manifest.Package{Name: pkgDesired.Name, Version: pkgDesired.Version})
		case strings.Compare(pkgDesired.Version, pkgActual.Version) != 0:
			p.log.Infof("desired package %s version %s does not match actual version %s",
				pkgDesired.Name, pkgDesired.Version, pkgActual.Version)
		default:
			p.log.Infof("desired package %s is ok, installed", pkgDesired.Name)
		}
	}

	if len(missing) > 0 {
		p.log.Infof("%s is missing %d packages, installing", p.manifest.ID, len(missing))
		if err := pkgs.Update(); err != nil {
			return errors.Wrapf(err, "error update packages on %s", p.manifest.ID)
		}
		if err := pkgs.Install(missing...); err != nil {
			return errors.Wrapf(err, "error installing packages on %s", p.manifest.ID)
		}
	}

	// there are tests that cover these data values.
	data := map[string]string{
		files.FileTemplateKeyLastModifiedDate: time.Now().UTC().Format(time.RFC3339),
	}
	// copy values from parameters to data map, for used by templates
	for k, v := range p.manifest.Parameters {
		data[k] = v
	}

	p.log.Infof("rendering and copying template with %v", data)
	fm := files.New(p.log, p.manifest, p.ssh)
	changedPackages, err := fm.RenderAndTransfer(data)
	if err != nil {
		return errors.Wrap(err, "error rendering files")
	}
	p.log.Infof("changed packages %+v", changedPackages)

	// apply file modes and ownership
	p.log.Info("applying permissions to files")
	if err := fm.ApplyPermissions(); err != nil {
		return errors.Wrap(err, "error applying permissions")
	}

	// now that packages are reconcile, files reconciled, restart services
	// only restart services that had packaged with changes
	for _, pkg := range changedPackages {
		if pkg.Kind == manifest.PackageKindService {
			if err := pkgs.RestartService(pkg.Name); err != nil {
				return errors.Wrapf(err, "error restarting service %s on %s", pkg.Name, p.manifest.ID)
			}
		}
	}

	// Check status for the services we expect to be running. Docker does
	// not have systemd running, so we use service(system V init, which is running in
	// docker) when the backed is docker.
	// TODO: move service status into backed, add to interface
	for _, pkg := range p.manifest.Packages {
		if pkg.Kind != manifest.PackageKindService {
			continue
		}
		cmd := "systemctl status %s"
		if p.manifest.Provider == manifest.ProviderBackendDocker {
			cmd = "service %s status"
		}
		// service name and package name might not be the same thing.
		// They work for now with the current manifests, but a future
		// improvement would be to add a service name override or something fancier.
		out, err := p.ssh.Execf(cmd, pkg.Name)
		if err != nil {
			p.log.Infof("warning: unable to get status of service out: '%s', pkage name: %s", out, pkg.Name)
		} else {
			p.log.Infof("service status out: '%s', package name: %s", out, pkg.Name)
		}
	}
	return nil
}

// Remove removes packages and files installed by reconcile
// context parameter is not yet used
// purge is passed to packages to purge package instead of just remove
func (p *ProviderReconciler) Remove(_ context.Context, purge bool) error {
	p.log.Infof("remove")

	p.log.Info("spew manifest", spew.Sdump(p.manifest))
	remove := make([]manifest.Package, 0, len(p.manifest.Packages))
	pkgs := packages.NewPackages(p.log, p.manifest, p.ssh)
	pkglist, err := pkgs.Query()
	if err != nil {
		return errors.Wrap(err, "error getting packages from container")
	}
	p.log.Infof("pkgs %d", len(pkglist))

	// diff the desired and actual packages, make a list of packages to install
	for _, pkg := range p.manifest.Packages {
		_, ok := pkglist[pkg.Name]
		if ok {
			p.log.Infof("package %s exists on remote, will remove, %s",
				pkg.Name, p.manifest.ID)
			remove = append(remove, manifest.Package{Name: pkg.Name, Version: pkg.Version})
		}
	}

	if len(remove) > 0 {
		if err := pkgs.Remove(purge, remove...); err != nil {
			p.log.Infof("remove package error, err: %v", err)
		}
		p.log.Infof("remove packages")
		fm := files.New(p.log, p.manifest, p.ssh)
		if err := fm.Remove(remove...); err != nil {
			return errors.Wrap(err, "error rendering files")
		}
	}
	return nil
}
