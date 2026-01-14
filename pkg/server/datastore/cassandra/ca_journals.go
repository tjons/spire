package cassandra

import (
	"context"
	"errors"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
	"github.com/sirupsen/logrus"
	"github.com/spiffe/spire/pkg/common/telemetry"
	"github.com/spiffe/spire/pkg/server/datastore"
	"github.com/spiffe/spire/proto/private/server/journal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type CAJournal struct {
	ID                    uint
	ActiveX509AuthorityID string
	JournalData           []byte
}

func (p *plugin) nextCAJournalID(ctx context.Context) (uint, error) {
	createQ := `SELECT MAX(id) FROM ca_journals ALLOW FILTERING`
	var maxID uint
	if err := p.db.session.Query(createQ).Scan(&maxID); err != nil {
		return 0, err
	}
	return maxID + 1, nil
}

func (p *plugin) SetCAJournal(ctx context.Context, caJournal *datastore.CAJournal) (*datastore.CAJournal, error) {
	if err := validateCAJournal(caJournal); err != nil {
		return nil, err
	}

	// TODO(tjons): handle quorum
	if caJournal.ID == 0 {
		return p.createCAJournal(ctx, caJournal)
	}

	return p.updateCAJournal(ctx, caJournal)
}

func (p *plugin) updateCAJournal(ctx context.Context, caJournal *datastore.CAJournal) (*datastore.CAJournal, error) {
	updateQ := `UPDATE ca_journals SET
		active_x509_authority_id = ?,
		data = ?,
		updated_at = toTimestamp(now())
		WHERE id = ? IF EXISTS`

	res := make(map[string]any)
	applied, err := p.db.session.Query(updateQ,
		caJournal.ActiveX509AuthorityID,
		caJournal.Data,
		caJournal.ID,
	).MapScanCAS(res)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update CA journal: %v", err)
	}

	if !applied {
		return nil, status.Error(codes.NotFound, NotFoundErr.Error())
	}

	return fetchCAJournalByID(ctx, caJournal.ID, p.db.session)
}

func (p *plugin) createCAJournal(ctx context.Context, caJournal *datastore.CAJournal) (*datastore.CAJournal, error) {
	nextId, err := p.nextCAJournalID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get next CA journal ID: %v", err)
	}

	createQ := `INSERT INTO ca_journals (
		id,
		active_x509_authority_id,
		data,
		created_at,
		updated_at
	) VALUES (?, ?, ?, toTimestamp(now()), toTimestamp(now())) IF NOT EXISTS`

	if err := p.db.session.Query(createQ,
		nextId,
		caJournal.ActiveX509AuthorityID,
		caJournal.Data,
	).Exec(); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to insert CA journal: %v", err)
	}

	caJournal.ID = nextId

	return caJournal, nil
}

func fetchCAJournalByID(ctx context.Context, id uint, s *gocql.Session) (*datastore.CAJournal, error) {
	findQ := `SELECT
		id,
		active_x509_authority_id,
		data
		FROM ca_journals
		WHERE id = ?`

	var caJournal datastore.CAJournal
	if err := s.Query(findQ, id).Scan(
		&caJournal.ID,
		&caJournal.ActiveX509AuthorityID,
		&caJournal.Data,
	); err != nil {
		return nil, err
	}

	return &caJournal, nil
}

func fetchCAJournalByActiveX509AuthorityID(ctx context.Context, activeX509AuthorityID string, s *gocql.Session) (*datastore.CAJournal, error) {
	findQ := `SELECT
		id,
		active_x509_authority_id,
		data
		FROM ca_journals
		WHERE active_x509_authority_id = ? LIMIT 1`

	var caJournal datastore.CAJournal
	if err := s.Query(findQ, activeX509AuthorityID).Scan(
		&caJournal.ID,
		&caJournal.ActiveX509AuthorityID,
		&caJournal.Data,
	); err != nil {
		return nil, err
	}

	return &caJournal, nil
}

func (p *plugin) FetchCAJournal(ctx context.Context, activeX509AuthorityID string) (*datastore.CAJournal, error) {
	if activeX509AuthorityID == "" {
		return nil, status.Error(codes.InvalidArgument, "active X509 authority ID is required")
	}

	j, err := fetchCAJournalByActiveX509AuthorityID(ctx, activeX509AuthorityID, p.db.session)
	if err != nil {
		if errors.Is(err, gocql.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return j, nil
}

func (p *plugin) PruneCAJournals(ctx context.Context, allAuthoritiesExpireBefore int64) error {
	journals, err := listCAJournals(ctx, p.db.session)
	if err != nil {
		return err
	}

checkAuthorities:
	for _, model := range journals {
		entries := new(journal.Entries)
		if err := proto.Unmarshal(model.Data, entries); err != nil {
			return status.Errorf(codes.Internal, "unable to unmarshal entries from CA journal record: %v", err)
		}

		for _, x509CA := range entries.X509CAs {
			if x509CA.NotAfter > allAuthoritiesExpireBefore {
				continue checkAuthorities
			}
		}

		for _, jwtKey := range entries.JwtKeys {
			if jwtKey.NotAfter > allAuthoritiesExpireBefore {
				continue checkAuthorities
			}
		}

		if err := deleteCAJournal(p.db.session, model.ID); err != nil {
			return status.Errorf(codes.Internal, "failed to delete CA journal: %v", err)
		}

		p.log.WithFields(logrus.Fields{
			telemetry.CAJournalID: model.ID,
		}).Info("Pruned stale CA journal record")
	}

	return nil
}

func deleteCAJournal(s *gocql.Session, id uint) error {
	deleteQ := `DELETE FROM ca_journals WHERE id = ? IF EXISTS`

	if err := s.Query(deleteQ, id).Exec(); err != nil {
		return status.Errorf(codes.Internal, "failed to delete CA journal: %v", err)
	}

	return nil
}

func (p *plugin) ListCAJournalsForTesting(ctx context.Context) ([]*datastore.CAJournal, error) {
	return listCAJournals(ctx, p.db.session)
}

func listCAJournals(ctx context.Context, s *gocql.Session) ([]*datastore.CAJournal, error) {
	listQ := `SELECT
		id,
		active_x509_authority_id,
		data
		FROM ca_journals`

	var caJournals []*datastore.CAJournal
	scanner := s.Query(listQ).Iter().Scanner()
	for scanner.Next() {
		caJournal := new(datastore.CAJournal)

		if err := scanner.Scan(
			&caJournal.ID,
			&caJournal.ActiveX509AuthorityID,
			&caJournal.Data,
		); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to scan CA journal: %v", err)
		}
		caJournals = append(caJournals, caJournal)
	}
	if err := scanner.Err(); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to iterate CA journals: %v", err)
	}

	return caJournals, nil
}

// TODO(tjons): copied from pkg/server/datastore/sqlstore/sqlstore.go:4935. unify this.
func validateCAJournal(caJournal *datastore.CAJournal) error {
	if caJournal == nil {
		return status.Error(codes.InvalidArgument, "ca journal is required")
	}

	return nil
}
