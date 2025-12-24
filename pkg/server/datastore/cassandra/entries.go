package cassandra

import (
	"context"
	"time"

	gocql "github.com/gocql/gocql"
	"github.com/gogo/status"
	"github.com/spiffe/spire/pkg/server/datastore"
	"github.com/spiffe/spire/proto/spire/common"
	"google.golang.org/grpc/codes"
)

type Selector struct {
	Type  string
	Value string
}

type RegistrationEntry struct {
	CreatedAt             time.Time
	UpdatedAt             time.Time
	EntryID               gocql.UUID
	SpiffeID              string
	ParentID              string
	Admin                 bool
	Downstream            bool
	TTL                   int32
	Expiry                int64
	RevisionNumber        int64
	StoreSVID             bool
	Hint                  string
	JWTSVIDTTL            int32
	Selectors             []Selector
	DNSNames              []string
	FederatedTrustDomains []string
}

func (p *plugin) CountRegistrationEntries(context.Context, *datastore.CountRegistrationEntriesRequest) (int32, error) {
	return 0, NotImplementedErr
}

func (p *plugin) CreateRegistrationEntry(ctx context.Context, entry *common.RegistrationEntry) (newEntry *common.RegistrationEntry, err error) {
	entryID, err := gocql.ParseUUID(entry.EntryId)
	if err != nil {
		return nil, newWrappedCassandraError(err)
	}

	if (entryID == gocql.UUID{}) {
		entryID, err = gocql.RandomUUID()
		if err != nil {
			return nil, newWrappedCassandraError(err)
		}
	}

	newRegisteredEntry := RegistrationEntry{
		EntryID:               entryID,
		SpiffeID:              entry.SpiffeId,
		ParentID:              entry.ParentId,
		Admin:                 entry.Admin,
		Downstream:            entry.Downstream,
		TTL:                   entry.X509SvidTtl,
		Expiry:                entry.EntryExpiry,
		RevisionNumber:        1,
		StoreSVID:             entry.StoreSvid,
		Hint:                  entry.Hint,
		JWTSVIDTTL:            entry.JwtSvidTtl,
		FederatedTrustDomains: entry.FederatesWith,
		DNSNames:              entry.DnsNames,
	}

	createEntryQuery := `
		INSERT INTO registered_entries (
			created_at,
			updated_at,
			entry_id,
			spiffe_id,
			parent_id,
			admin,
			downstream,
			ttl,
			expiry,
			revision_number,
			store_svid,
			hint,
			jwt_svid_ttl,
			dns_names,
			federated_trust_domains,
			selector_type,
			selector_value
		) VALUES (toTimestamp(now()), toTimestamp(now()), ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?, ?)
	`

	commonVals := []any{
		newRegisteredEntry.EntryID,
		newRegisteredEntry.SpiffeID,
		newRegisteredEntry.ParentID,
		newRegisteredEntry.Admin,
		newRegisteredEntry.Downstream,
		newRegisteredEntry.TTL,
		newRegisteredEntry.Expiry,
		newRegisteredEntry.StoreSVID,
		newRegisteredEntry.Hint,
		newRegisteredEntry.JWTSVIDTTL,
		newRegisteredEntry.DNSNames,
		newRegisteredEntry.FederatedTrustDomains,
	}

	b := p.db.session.Batch(gocql.LoggedBatch)
	for _, selector := range entry.Selectors {
		b.Query(createEntryQuery, append(commonVals, selector.Type, selector.Value)...)
	}

	if err := p.db.session.ExecuteBatch(b); err != nil {
		return nil, newWrappedCassandraError(err)
	}

	return nil, NotImplementedErr
}

func (p *plugin) CreateOrReturnRegistrationEntry(context.Context, *common.RegistrationEntry) (*common.RegistrationEntry, bool, error) {
	return nil, false, NotImplementedErr
}

func (p *plugin) DeleteRegistrationEntry(ctx context.Context, entryID string) (*common.RegistrationEntry, error) {
	return nil, NotImplementedErr
}

func (p *plugin) FetchRegistrationEntry(ctx context.Context, entryID string) (*common.RegistrationEntry, error) {
	return nil, NotImplementedErr
}

func (p *plugin) FetchRegistrationEntries(ctx context.Context, entryIDs []string) (map[string]*common.RegistrationEntry, error) {
	return nil, NotImplementedErr
}

func (p *plugin) ListRegistrationEntries(ctx context.Context, req *datastore.ListRegistrationEntriesRequest) (*datastore.ListRegistrationEntriesResponse, error) {
	if req.Pagination != nil && req.Pagination.PageSize == 0 {
		return nil, status.Error(codes.InvalidArgument, "cannot paginate with pagesize = 0")
	}
	if req.BySelectors != nil && len(req.BySelectors.Selectors) == 0 {
		return nil, status.Error(codes.InvalidArgument, "cannot list by empty selector set")
	}

	return nil, NotImplementedErr
}

func (p *plugin) PruneRegistrationEntries(ctx context.Context, expiresBefore time.Time) error {
	return NotImplementedErr
}

func (p *plugin) UpdateRegistrationEntry(context.Context, *common.RegistrationEntry, *common.RegistrationEntryMask) (*common.RegistrationEntry, error) {
	return nil, NotImplementedErr
}
