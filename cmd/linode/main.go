package main

import (
	"context"
	"fmt"
	"strconv"

	"github.com/linode/linodego"
	"golang.org/x/oauth2"

	"log"
	"net/http"
	"os"
)

func main() {
	apiKey, ok := os.LookupEnv("LINODE_TOKEN")
	if !ok {
		log.Fatal("Could not find LINODE_TOKEN, please assert it is set.")
	}
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: apiKey})

	oauth2Client := &http.Client{
		Transport: &oauth2.Transport{
			Source: tokenSource,
		},
	}

	linodeClient := linodego.NewClient(oauth2Client)
	linodeClient.SetDebug(true)

	instanceID, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	res, err := linodeClient.GetInstance(context.Background(), instanceID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%v", res)
}
