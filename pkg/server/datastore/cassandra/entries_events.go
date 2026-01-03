package cassandra

import (
	"context"
	"time"

	"github.com/spiffe/spire/pkg/server/datastore"
)

func (p *plugin) ListRegistrationEntryEvents(ctx context.Context, req *datastore.ListRegistrationEntryEventsRequest) (*datastore.ListRegistrationEntryEventsResponse, error) {
	return nil, NotImplementedErr

}
func (p *plugin) PruneRegistrationEntryEvents(ctx context.Context, olderThan time.Duration) error {
	return NotImplementedErr
}
func (p *plugin) FetchRegistrationEntryEvent(ctx context.Context, eventID uint) (*datastore.RegistrationEntryEvent, error) {
	return nil, NotImplementedErr

}
func (p *plugin) CreateRegistrationEntryEventForTesting(ctx context.Context, event *datastore.RegistrationEntryEvent) error {
	return NotImplementedErr
}
func (p *plugin) DeleteRegistrationEntryEventForTesting(ctx context.Context, eventID uint) error {
	return NotImplementedErr
}
