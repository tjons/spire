package cassandra

import (
	"context"
	"time"

	"github.com/spiffe/spire/pkg/server/datastore"
)

func (db *plugin) ListAttestedNodeEvents(ctx context.Context, req *datastore.ListAttestedNodeEventsRequest) (*datastore.ListAttestedNodeEventsResponse, error) {
	return nil, NotImplementedErr

}
func (db *plugin) PruneAttestedNodeEvents(ctx context.Context, olderThan time.Duration) error {
	return NotImplementedErr
}
func (db *plugin) FetchAttestedNodeEvent(ctx context.Context, eventID uint) (*datastore.AttestedNodeEvent, error) {
	return nil, NotImplementedErr

}
func (db *plugin) CreateAttestedNodeEventForTesting(ctx context.Context, event *datastore.AttestedNodeEvent) error {
	return NotImplementedErr

}
func (db *plugin) DeleteAttestedNodeEventForTesting(ctx context.Context, eventID uint) error {
	return NotImplementedErr
}
