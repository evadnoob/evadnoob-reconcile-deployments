package generate

import (
	"os"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"

	"slack-reconcile-deployments/cmd/flags"
	"slack-reconcile-deployments/internal/generate"
	"slack-reconcile-deployments/internal/manifest"
)

// New returns the generate command
func New() *cli.Command {
	return &cli.Command{
		Name:  "generate",
		Usage: `generate manifest files from templates, renders to stdout`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "provider",
				Usage: `use provider to generate a manifest for different providers.`,
				Value: string(manifest.ProviderBackendSlack),
				Action: func(context *cli.Context, s string) error {
					// validate provider argument values
					switch manifest.ProviderBackend(s) {
					case manifest.ProviderBackendDocker,
						manifest.ProviderBackendEC2,
						manifest.ProviderBackendSlack:
						// ok, valid
						return nil
					default:
						return errors.Errorf("invalid provider: %s", s)
					}
				},
			},
			&cli.StringFlag{
				Name: flags.FlagNameUniqueIDFormat,
				Usage: `use unique id format to generate a manifest id. ` +
					`Valid values are random and ulid. Use ulids if you'd to ` +
					`have lexicographically sortable ids.`,
				Value: string(manifest.UniqueIDFormatRandom),
				Action: func(context *cli.Context, s string) error {
					switch manifest.UniqueIDFormat(s) {
					case manifest.UniqueIDFormatRandom,
						manifest.UniqueIDFormatULID:
						// ok, valid
						return nil
					default:
						return errors.Errorf("invalid unique id format: %s", s)
					}
				},
			},
		},
		Action: func(c *cli.Context) error {
			return generate.Run(os.Stdout,
				manifest.UniqueIDFormat(c.String(flags.FlagNameUniqueIDFormat)))
		},
	}
}
