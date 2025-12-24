package cassandra

import (
	"context"

	"github.com/spiffe/spire/pkg/server/datastore"
)

func (p *plugin) SetCAJournal(ctx context.Context, caJournal *datastore.CAJournal) (*datastore.CAJournal, error) {
	return nil, NotImplementedErr

}

func (p *plugin) FetchCAJournal(ctx context.Context, activeX509AuthorityID string) (*datastore.CAJournal, error) {
	return nil, NotImplementedErr

}

func (p *plugin) PruneCAJournals(ctx context.Context, allCAsExpireBefore int64) error {
	return NotImplementedErr
}

func (p *plugin) ListCAJournalsForTesting(ctx context.Context) ([]*datastore.CAJournal, error) {
	return nil, NotImplementedErr
}
