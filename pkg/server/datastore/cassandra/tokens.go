package cassandra

import (
	"context"
	"time"

	"github.com/spiffe/spire/pkg/server/datastore"
)

func (p *plugin) CreateJoinToken(context.Context, *datastore.JoinToken) error {
	return NotImplementedErr
}

func (p *plugin) DeleteJoinToken(ctx context.Context, token string) error {
	return NotImplementedErr
}

func (p *plugin) FetchJoinToken(ctx context.Context, token string) (*datastore.JoinToken, error) {
	return nil, NotImplementedErr
}

func (p *plugin) PruneJoinTokens(context.Context, time.Time) error {
	return NotImplementedErr
}
