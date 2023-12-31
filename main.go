package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"

	"slack-reconcile-deployments/cmd/generate"
	"slack-reconcile-deployments/cmd/reconcile"
	"slack-reconcile-deployments/cmd/verify"
)

func main() {
	app := &cli.App{
		Name:  "slack-reconcile-deployments",
		Usage: "slack-reconcile-deployments",
		Commands: []*cli.Command{
			reconcile.New(),
			generate.New(),
			verify.New(),
		},
		Flags: []cli.Flag{},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
