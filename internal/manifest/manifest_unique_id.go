package manifest

import (
	"crypto/rand"
	"fmt"

	"github.com/oklog/ulid/v2"
	"github.com/pkg/errors"
)

type UniqueIDFormat string

// Unique id format for manifest ids
const (
	UniqueIDFormatRandom = UniqueIDFormat("random")
	UniqueIDFormatULID   = UniqueIDFormat("ulid")
)

// NewID creates a new unique id for use in manifests.
// IDs for manifests are used to identify existing hosts
// in some backend providers like ec2.
func NewID(format UniqueIDFormat) (string, error) {
	switch format {
	case UniqueIDFormatULID:
		id, err := ulid.New(ulid.Now(), ulid.DefaultEntropy())
		return id.String(), errors.Wrap(err, "error creating new id")
	default:
		id, err := randID(5)
		return fmt.Sprintf("%x", id), errors.Wrap(err, "error creating new id")
	}
}

// generate a random id similar to openssl rand -hex 5
func randID(length int) ([]byte, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return nil, errors.Wrap(err, "error reading random bytes")
	}
	return b, nil
}
