package cassandra

import (
	"context"
	"strings"
	"time"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
	"github.com/spiffe/spire/pkg/server/datastore"
)

func (p *plugin) ListRegistrationEntryEvents(ctx context.Context, req *datastore.ListRegistrationEntryEventsRequest) (*datastore.ListRegistrationEntryEventsResponse, error) {
	b := strings.Builder{}
	b.WriteString("SELECT id, entry_id FROM registration_entry_events ")
	var args []any

	switch {
	case req.GreaterThanEventID > 0 && req.LessThanEventID > 0:
		b.WriteString("WHERE id > ? AND id < ? ")
		args = append(args, req.GreaterThanEventID, req.LessThanEventID)
	case req.LessThanEventID > 0:
		b.WriteString("WHERE id < ? ")
		args = append(args, req.LessThanEventID)
	case req.GreaterThanEventID > 0:
		b.WriteString("WHERE id > ? ")
		args = append(args, req.GreaterThanEventID)
	}
	b.WriteString(" ALLOW FILTERING")

	iter := p.db.session.Query(b.String(), args...).Iter()
	scanner := iter.Scanner()
	events := make([]datastore.RegistrationEntryEvent, 0)

	for scanner.Next() {
		event := new(datastore.RegistrationEntryEvent)
		if err := scanner.Scan(
			&event.EventID,
			&event.EntryID,
		); err != nil {
			return nil, err
		}
		events = append(events, *event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	resp := &datastore.ListRegistrationEntryEventsResponse{
		Events: events,
	}

	return resp, nil

}

func (p *plugin) createRegistrationEntryEvent(ctx context.Context, event *datastore.RegistrationEntryEvent) error {
	if event.EventID == 0 {
		nextID, err := getNextRegistrationEntryEventID(ctx, p.db.session)
		if err != nil {
			return err
		}
		event.EventID = nextID
	}

	q := `INSERT INTO registration_entry_events (id, entry_id, created_at, updated_at) VALUES (?, ?, toTimestamp(now()), toTimestamp(now()))`
	if err := p.db.session.Query(q,
		event.EventID,
		event.EntryID,
	).ExecContext(ctx); err != nil {
		return err
	}

	return nil
}

func getNextRegistrationEntryEventID(ctx context.Context, s *gocql.Session) (uint, error) {
	q := `SELECT max(id) FROM registration_entry_events ALLOW FILTERING`

	var maxID *uint
	if err := s.Query(q).ScanContext(ctx, &maxID); err != nil {
		return 0, err
	}
	if maxID == nil {
		return 1, nil
	}
	return *maxID + 1, nil
}

func (p *plugin) PruneRegistrationEntryEvents(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)

	idsQ := `SELECT id, entry_id FROM registration_entry_events WHERE created_at < ? ALLOW FILTERING`
	scanner := p.db.session.Query(idsQ, cutoff).Iter().Scanner()

	var events []datastore.RegistrationEntryEvent
	for scanner.Next() {
		var event datastore.RegistrationEntryEvent
		if err := scanner.Scan(&event.EventID, &event.EntryID); err != nil {
			return err
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	b := p.db.session.Batch(gocql.LoggedBatch)
	deleteQ := `DELETE FROM registration_entry_events WHERE entry_id = ? AND id = ?`
	for _, event := range events {
		b.Entries = append(b.Entries, gocql.BatchEntry{
			Stmt:       deleteQ,
			Args:       []any{event.EntryID, event.EventID},
			Idempotent: true,
		})
	}
	if err := b.ExecContext(ctx); err != nil {
		return err
	}

	return nil
}

func (p *plugin) FetchRegistrationEntryEvent(ctx context.Context, eventID uint) (*datastore.RegistrationEntryEvent, error) {
	q := `SELECT id, entry_id FROM registration_entry_events WHERE id = ?`

	var event datastore.RegistrationEntryEvent
	if err := p.db.session.Query(q, eventID).ScanContext(ctx,
		&event.EventID,
		&event.EntryID,
	); err != nil {
		if err == gocql.ErrNotFound {
			return nil, NotFoundErr
		}
		return nil, err
	}

	return &event, nil

}
func (p *plugin) CreateRegistrationEntryEventForTesting(ctx context.Context, event *datastore.RegistrationEntryEvent) error {
	return p.createRegistrationEntryEvent(ctx, event)
}
func (p *plugin) DeleteRegistrationEntryEventForTesting(ctx context.Context, eventID uint) error {
	q := `DELETE FROM registration_entry_events WHERE id = ?`
	return p.db.session.Query(q, eventID).ExecContext(ctx)
}
