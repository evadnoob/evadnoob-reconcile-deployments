package verify

import (
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"golang.org/x/sync/errgroup"

	"slack-reconcile-deployments/cmd/flags"
)

// New returns the verify command
func New() *cli.Command {

	return &cli.Command{
		Name: "verify",
		Usage: `verify reconcile state by running http get against each target host. ` +
			`Output is similar to curl -v.`,
		Flags: []cli.Flag{
			flags.FlagQuiet,
			flags.FlagConcurrency,
		},
		Action: func(c *cli.Context) error {
			errgrp := errgroup.Group{}
			// setting limit to keep output organized.
			// could set this higher by if you don't
			// mind messy output.
			errgrp.SetLimit(c.Int(flags.FlagNameConcurrency))

			if c.NArg() == 0 {
				return errors.Errorf("missing required hostname arguments")
			}
			for _, hostname := range c.Args().Slice() {
				// capture/copy loop variable for go routine
				hostname := hostname
				errgrp.Go(func() error {
					requestURL := fmt.Sprintf("http://%s", hostname)
					res, err := http.Get(requestURL)
					if err != nil {
						return errors.Wrapf(err, "error getting %s", requestURL)
					}

					b, err := io.ReadAll(res.Body)
					if err != nil {
						return errors.Wrapf(err, "error reading response body")
					}

					fmt.Printf("> %s %s\n", res.Proto, res.Status)
					for k, v := range res.Header {
						fmt.Printf("> %s %s\n", k, v)
					}
					fmt.Printf("%s\n", string(b))
					return nil
				})
			}
			return errgrp.Wait()
		},
	}
}
