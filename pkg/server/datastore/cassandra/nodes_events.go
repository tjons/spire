package cassandra

import (
	"context"
	"strings"
	"time"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
	"github.com/spiffe/spire/pkg/server/datastore"
)

func (p *plugin) ListAttestedNodeEvents(ctx context.Context, req *datastore.ListAttestedNodeEventsRequest) (*datastore.ListAttestedNodeEventsResponse, error) {
	b := strings.Builder{}
	b.WriteString("SELECT event_id, spiffe_id FROM attested_node_entries_events ")
	var args []any

	switch {
	case req.GreaterThanEventID > 0 && req.LessThanEventID > 0:
		b.WriteString("WHERE event_id > ? AND event_id < ? ")
		args = append(args, req.GreaterThanEventID, req.LessThanEventID)
	case req.LessThanEventID > 0:
		b.WriteString("WHERE event_id < ? ")
		args = append(args, req.LessThanEventID)
	case req.GreaterThanEventID > 0:
		b.WriteString("WHERE event_id > ? ")
		args = append(args, req.GreaterThanEventID)
	}
	b.WriteString(" ALLOW FILTERING")

	iter := p.db.session.Query(b.String(), args...).Iter()
	scanner := iter.Scanner()
	events := make([]datastore.AttestedNodeEvent, 0)

	for scanner.Next() {
		var (
			eventID  uint
			spiffeID string
		)

		if err := scanner.Scan(
			&eventID,
			&spiffeID,
		); err != nil {
			return nil, err
		}

		events = append(events, datastore.AttestedNodeEvent{
			EventID:  eventID,
			SpiffeID: spiffeID,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &datastore.ListAttestedNodeEventsResponse{
		Events: events,
	}, nil
}

func (p *plugin) createAttestedNodeEvent(ctx context.Context, event *datastore.AttestedNodeEvent) error {
	if event.EventID == 0 {
		nextID, err := getNextAttestedNodeEventID(ctx, p.db.session)
		if err != nil {
			return err
		}
		event.EventID = nextID
	}

	query := `INSERT INTO attested_node_entries_events (event_id, created_at, updated_at, spiffe_id) VALUES (?, toTimestamp(now()), toTimestamp(now()), ?)`
	if err := p.db.session.Query(query,
		event.EventID,
		event.SpiffeID,
	).ExecContext(ctx); err != nil {
		return newCassandraError("failed to create attested node event: %w", err)
	}

	return nil
}

func getNextAttestedNodeEventID(ctx context.Context, s *gocql.Session) (uint, error) {
	q := `SELECT max(event_id) FROM attested_node_entries_events ALLOW FILTERING`

	var maxID *uint
	if err := s.Query(q).ScanContext(ctx, &maxID); err != nil && err != gocql.ErrNotFound {
		return 0, newCassandraError("failed to get max attested node event ID: %w", err)
	}
	if maxID == nil {
		return 1, nil
	}

	return *maxID + 1, nil
}

func (p *plugin) PruneAttestedNodeEvents(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)

	idsQ := `SELECT event_id, spiffe_id FROM attested_node_entries_events WHERE created_at < ? ALLOW FILTERING`
	scanner := p.db.session.Query(idsQ, cutoff).Iter().Scanner()

	var events []datastore.AttestedNodeEvent
	for scanner.Next() {
		var event datastore.AttestedNodeEvent
		if err := scanner.Scan(&event.EventID, &event.SpiffeID); err != nil {
			return err
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	b := p.db.session.Batch(gocql.LoggedBatch)
	deleteQ := `DELETE FROM attested_node_entries_events WHERE spiffe_id = ? AND event_id = ?`
	for _, event := range events {
		b.Entries = append(b.Entries, gocql.BatchEntry{
			Stmt:       deleteQ,
			Args:       []any{event.SpiffeID, event.EventID},
			Idempotent: true,
		})
	}
	if err := b.ExecContext(ctx); err != nil {
		return err
	}

	return nil
}

func (p *plugin) FetchAttestedNodeEvent(ctx context.Context, eventID uint) (*datastore.AttestedNodeEvent, error) {
	q := `SELECT event_id, spiffe_id FROM attested_node_entries_events WHERE event_id = ?`

	var event datastore.AttestedNodeEvent
	if err := p.db.session.Query(q, eventID).ScanContext(ctx,
		&event.EventID,
		&event.SpiffeID,
	); err != nil {
		if err == gocql.ErrNotFound {
			return nil, NotFoundErr
		}
		return nil, newCassandraError("failed to fetch attested node event: %w", err)
	}

	return &event, nil
}

func (p *plugin) CreateAttestedNodeEventForTesting(ctx context.Context, event *datastore.AttestedNodeEvent) error {
	return p.createAttestedNodeEvent(ctx, event)
}

func (s *plugin) DeleteAttestedNodeEventForTesting(ctx context.Context, eventID uint) error {
	findEventQ := `SELECT spiffe_id FROM attested_node_entries_events WHERE event_id = ?`

	var spiffeID string
	if err := s.db.session.Query(findEventQ, eventID).ScanContext(ctx, &spiffeID); err != nil {
		if err == gocql.ErrNotFound {
			return NotFoundErr
		}
		return newCassandraError("failed to find attested node event for deletion: %w", err)
	}

	deleteQ := `DELETE FROM attested_node_entries_events WHERE spiffe_id = ? AND event_id = ?`
	if err := s.db.session.Query(deleteQ, spiffeID, eventID).ExecContext(ctx); err != nil {
		return newCassandraError("failed to delete attested node event: %w", err)
	}

	return nil
}
