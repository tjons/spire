package cassandra

import (
	"context"
	"time"

	"github.com/spiffe/spire/pkg/server/datastore"
)

func (db *plugin) CreateJoinToken(context.Context, *datastore.JoinToken) error {
	return NotImplementedErr
}

func (db *plugin) DeleteJoinToken(ctx context.Context, token string) error {
	return NotImplementedErr
}

func (db *plugin) FetchJoinToken(ctx context.Context, token string) (*datastore.JoinToken, error) {
	return nil, NotImplementedErr
}

func (db *plugin) PruneJoinTokens(context.Context, time.Time) error {
	return NotImplementedErr
}
