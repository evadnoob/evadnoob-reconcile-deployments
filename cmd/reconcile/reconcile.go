package reconcile

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"golang.org/x/sync/errgroup"

	"slack-reconcile-deployments/cmd/flags"
	"slack-reconcile-deployments/internal/logging"
	"slack-reconcile-deployments/internal/manifest"
	"slack-reconcile-deployments/internal/reconcile"
	"slack-reconcile-deployments/internal/reconcile/backend"
)

// New returns the reconcile command
func New() *cli.Command {
	const maxReconcileManifests = 10

	return &cli.Command{
		Name: "reconcile",
		Usage: `reconciles remote hosts with state described in manifest files. ` +
			`go run main.go reconcile --manifests/manifest1.yaml ` +
			`--manifests/manifest2.yaml --packages packages.yaml`,
		Flags: []cli.Flag{
			flags.FlagConcurrency,
			flags.FlagManifest,
			flags.FlagPackages,
			flags.FlagTimeout,
			flags.FlagPassword,
			flags.FlagRemove,
			flags.FlagPurge,
		},
		Action: func(c *cli.Context) error {
			log := logging.New(c.App.Name, c.Bool(flags.FlagNameQuiet))
			manifestPaths := c.StringSlice("manifest")
			// prevent providing a large number of reconcile manifests. This
			// prevents either malicious or errors from causing reconcile
			// to try to connect to a large number of hosts
			if len(manifestPaths) > maxReconcileManifests {
				return errors.Errorf("manifest argument is over the limtit of %d", maxReconcileManifests)
			}
			packagesPath := c.String("packages")

			errgrp := errgroup.Group{}
			errgrp.SetLimit(c.Int(flags.FlagNameConcurrency))
			for i := range manifestPaths {
				// capture/copy loop variable for go routine
				i := i
				m, err := manifest.NewFromFile(manifestPaths[i], packagesPath)
				if err != nil {
					log.Errorf("error reading manifest file: %+v", err)
					return err
				}

				// uses functional options to set password on provider backend
				// not all providers user plain usernames and password, so
				// these options are dynamically set based on the provider.
				var options []func(reconciler backend.ProviderBackendReconciler)
				if m.Provider == manifest.ProviderBackendSlack {
					options = append(options, func(reconciler backend.ProviderBackendReconciler) {
						reconciler.WithOption("password", c.String("password"))
					})
				}

				// choose the reconcile operation, either reconcile or remove.
				// Remove is destructive and will delete files, remove packages
				reconcileOP := reconcile.Reconcile
				if c.Bool(flags.FlagNameRemove) {
					reconcileOP = reconcile.Remove
				}
				if c.Bool(flags.FlagNamePurge) {
					reconcileOP = reconcile.Purge
				}

				// start go routines one per manifest path, concurrency is limited to prevent
				// ddos the targets some future improvements could be to add some jitter, add
				// deployment groups, respond the quantity of reconciles etc. For now this is
				// approach is simple.
				errgrp.Go(func() error {
					timeout, err := time.ParseDuration(c.String(flags.FlagNameTimeout))
					if err != nil {
						return err
					}
					ctx, cancel := context.WithTimeout(c.Context, timeout)
					defer cancel()
					if err := reconcile.Run(ctx, log, m, reconcileOP, options...); err != nil {
						log.Errorf("error running reconcile for %s path: %s: %+v", m.ID, manifestPaths[i], err)
						return err
					}
					return nil
				})
			}
			return errgrp.Wait()
		},
	}
}
