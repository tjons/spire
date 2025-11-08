package cassandra

import (
	"context"
	"time"

	"github.com/spiffe/spire/pkg/server/datastore"
)

func (db *plugin) ListRegistrationEntryEvents(ctx context.Context, req *datastore.ListRegistrationEntryEventsRequest) (*datastore.ListRegistrationEntryEventsResponse, error) {
	return nil, NotImplementedErr

}
func (db *plugin) PruneRegistrationEntryEvents(ctx context.Context, olderThan time.Duration) error {
	return NotImplementedErr
}
func (db *plugin) FetchRegistrationEntryEvent(ctx context.Context, eventID uint) (*datastore.RegistrationEntryEvent, error) {
	return nil, NotImplementedErr

}
func (db *plugin) CreateRegistrationEntryEventForTesting(ctx context.Context, event *datastore.RegistrationEntryEvent) error {
	return NotImplementedErr
}
func (db *plugin) DeleteRegistrationEntryEventForTesting(ctx context.Context, eventID uint) error {
	return NotImplementedErr
}
