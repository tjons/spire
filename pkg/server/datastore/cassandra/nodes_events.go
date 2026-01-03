package cassandra

import (
	"context"
	"time"

	"github.com/spiffe/spire/pkg/server/datastore"
)

func (p *plugin) ListAttestedNodeEvents(ctx context.Context, req *datastore.ListAttestedNodeEventsRequest) (*datastore.ListAttestedNodeEventsResponse, error) {
	return nil, NotImplementedErr

}
func (p *plugin) PruneAttestedNodeEvents(ctx context.Context, olderThan time.Duration) error {
	return NotImplementedErr
}
func (p *plugin) FetchAttestedNodeEvent(ctx context.Context, eventID uint) (*datastore.AttestedNodeEvent, error) {
	return nil, NotImplementedErr

}
func (p *plugin) CreateAttestedNodeEventForTesting(ctx context.Context, event *datastore.AttestedNodeEvent) error {
	return NotImplementedErr

}
func (p *plugin) DeleteAttestedNodeEventForTesting(ctx context.Context, eventID uint) error {
	return NotImplementedErr
}
