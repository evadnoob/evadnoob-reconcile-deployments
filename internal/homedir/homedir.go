package homedir

import (
	"github.com/ory/dockertest/v3/docker/pkg/homedir"
)

// Get gets a user's home directory using ory/dockertest as convenience.
// This function is a skinny wrapper to abstract the dependency on ory/dockertest.
func Get() string {
	return homedir.Get()
}
