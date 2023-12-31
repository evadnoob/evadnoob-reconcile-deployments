package flags

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/urfave/cli/v2"
)

// maxConcurrency is the maximum number go routines for reconciling
const maxConcurrency = 3

// Flags for cli commands
const (
	FlagNameConcurrency    = "concurrency"
	FlagNameManifest       = "manifest"
	FlagNamePackages       = "packages"
	FlagNameTimeout        = "timeout"
	FlagNamePassword       = "password"
	FlagNameQuiet          = "quiet"
	FlagNameRemove         = "remove"
	FlagNamePurge          = "purge"
	FlagNameUniqueIDFormat = "unique-id-format"
)

// shared/common flags
var (
	FlagQuiet = &cli.BoolFlag{
		Name:  FlagNameQuiet,
		Usage: "quiet will log to a file instead of stdout",
		Value: false,
	}

	FlagConcurrency = &cli.IntFlag{
		Name:    FlagNameConcurrency,
		Aliases: []string{"c"},
		Usage: fmt.Sprintf("number of manifests to reconcile concurrently, not more than max concurrency %d",
			maxConcurrency),
		Value: 2,
		Action: func(c *cli.Context, concurrency int) error {
			// override concurrency if it is set over max concurrency, prevents overwhelming targets
			if concurrency > maxConcurrency {
				return c.Set(FlagNameConcurrency, strconv.Itoa(maxConcurrency))
			}
			return nil
		},
	}

	FlagManifest = &cli.StringSliceFlag{
		Name:     FlagNameManifest,
		Aliases:  []string{"m"},
		Usage:    "path to manifest files(multiple allowed)",
		Required: true,
	}

	FlagPackages = &cli.StringFlag{
		Name:     FlagNamePackages,
		Usage:    "path to packages file",
		Required: true,
	}

	FlagTimeout = &cli.StringFlag{
		Name:    FlagNameTimeout,
		Aliases: []string{"t"},
		Usage:   "timeout in duration, will be parsed by time.ParseDuration",
		Value:   "15m",
	}

	FlagPassword = &cli.StringFlag{
		Name:    FlagNamePassword,
		Aliases: []string{"p"},
		Usage:   "plain text password to be used for ssh auth. This flag disabled publickey auth",
		Action: func(c *cli.Context, s string) error {
			if s != "" {
				// make sure password does not have trailing slash
				_ = c.Set("password", strings.TrimSpace(s))
			}
			return nil
		},
	}

	FlagRemove = &cli.BoolFlag{
		Name:  FlagNameRemove,
		Usage: "remove operation, will cause reconcile to remove packages, use purge to fully remove",
		Value: false,
	}

	FlagPurge = &cli.BoolFlag{
		Name:  FlagNamePurge,
		Usage: "purge operation, will cause reconcile to purge packages and file and stop services",
		Value: false,
	}
)
