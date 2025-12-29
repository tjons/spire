package cassandra

import (
	"context"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
	"github.com/gogo/status"
	"github.com/sirupsen/logrus"
	"github.com/spiffe/spire/pkg/common/telemetry"
	"github.com/spiffe/spire/pkg/server/datastore"
	"github.com/spiffe/spire/proto/spire/common"
	"google.golang.org/grpc/codes"
)

type RegistrationEntry struct {
	CreatedAt             time.Time
	UpdatedAt             time.Time
	EntryID               string
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
	Selectors             []*common.Selector
	DNSNames              []string
	FederatedTrustDomains []string
}

func (p *plugin) CountRegistrationEntries(ctx context.Context, req *datastore.CountRegistrationEntriesRequest) (int32, error) {
	args := []any{}
	fields := []string{}
	operators := []string{}
	if len(req.ByParentID) > 0 {
		args = append(args, req.ByParentID)
		fields = append(fields, "parent_id")
		operators = append(operators, "=")
	}

	if len(req.BySpiffeID) > 0 {
		args = append(args, req.BySpiffeID)
		fields = append(fields, "spiffe_id")
		operators = append(operators, "=")
	}

	if req.ByDownstream != nil {
		args = append(args, *req.ByDownstream)
		fields = append(fields, "downstream")
		operators = append(operators, "=")
	}

	if req.ByFederatesWith != nil && len(req.ByFederatesWith.TrustDomains) > 0 {
		args = append(args, req.ByFederatesWith.TrustDomains)
		fields = append(fields, "federated_trust_domains")
		operators = append(operators, "CONTAINS")
	}

	if len(req.ByHint) > 0 {
		args = append(args, req.ByHint)
		fields = append(fields, "hint")
		operators = append(operators, "=")
	}

	if req.BySelectors != nil {
		// TODO(tjons): implement selector-based counting
		return 0, NotImplementedErr
	}

	b := strings.Builder{}
	b.WriteString("SELECT COUNT(*) FROM registered_entries")
	if len(fields) > 0 {
		b.WriteString(" WHERE ")
		for i, field := range fields {
			if i > 0 {
				b.WriteString(" AND ")
			}
			b.WriteString(field)
			b.WriteString(" ")
			b.WriteString(operators[i])
			b.WriteString(" ?")
		}
	}

	query := b.String()
	cqlQuery := p.db.session.Query(query, args...)
	cqlQuery.Consistency(gocql.LocalQuorum)

	var count int32
	if err := cqlQuery.Scan(&count); err != nil {
		return 0, newWrappedCassandraError(err)
	}

	return count, nil
}

// TODO(tjons): should this really be there with no validation? is this effectively unused?
func (p *plugin) CreateRegistrationEntry(ctx context.Context, entry *common.RegistrationEntry) (newEntry *common.RegistrationEntry, err error) {
	if err := validateRegistrationEntry(entry); err != nil {
		return nil, err
	}

	var entryID string

	if len(entry.EntryId) > 0 {
		entryID = entry.EntryId
	} else {
		uuid, err := gocql.RandomUUID()
		if err != nil {
			return nil, newWrappedCassandraError(err)
		}
		entryID = uuid.String()
	}

	newRegisteredEntry := RegistrationEntry{
		EntryID:               entryID,
		SpiffeID:              entry.SpiffeId,
		ParentID:              entry.ParentId,
		Admin:                 entry.Admin,
		Downstream:            entry.Downstream,
		TTL:                   entry.X509SvidTtl,
		Expiry:                entry.EntryExpiry,
		RevisionNumber:        0,
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
		) VALUES (toTimestamp(now()), toTimestamp(now()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	// TODO(tjons): consistency level?
	// is read consistency cheaper than write consistency?

	commonVals := []any{
		newRegisteredEntry.EntryID,
		newRegisteredEntry.SpiffeID,
		newRegisteredEntry.ParentID,
		newRegisteredEntry.Admin,
		newRegisteredEntry.Downstream,
		newRegisteredEntry.TTL,
		newRegisteredEntry.Expiry,
		newRegisteredEntry.RevisionNumber,
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

	// TODO(tjons): context management?
	if err := b.Exec(); err != nil {
		return nil, newWrappedCassandraError(err)
	}

	newRegisteredEntry.CreatedAt = time.Now() // TODO(tjons): this feels hacky?
	newRegisteredEntry.Selectors = entry.Selectors

	return registrationEntryModelToProto(&newRegisteredEntry), nil
}

func registrationEntryModelToProto(re *RegistrationEntry) *common.RegistrationEntry {
	r := &common.RegistrationEntry{
		EntryId:       re.EntryID,
		SpiffeId:      re.SpiffeID,
		ParentId:      re.ParentID,
		Admin:         re.Admin,
		Downstream:    re.Downstream,
		X509SvidTtl:   re.TTL,
		EntryExpiry:   re.Expiry,
		StoreSvid:     re.StoreSVID,
		Hint:          re.Hint,
		JwtSvidTtl:    re.JWTSVIDTTL,
		DnsNames:      re.DNSNames,
		FederatesWith: re.FederatedTrustDomains,
		CreatedAt:     re.CreatedAt.Unix(),
		Selectors:     re.Selectors,
	}

	return r
}

// Copied verbatim from pkg/server/datastore/sqlstore/sqlstore.go:39
var validEntryIDChars = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x002d, 0x002e, 1}, // - | .
		{0x0030, 0x0039, 1}, // [0-9]
		{0x0041, 0x005a, 1}, // [A-Z]
		{0x005f, 0x005f, 1}, // _
		{0x0061, 0x007a, 1}, // [a-z]
	},
	LatinOffset: 5,
}

// copied verbatim from pkg/server/datastore/sqlstore/sqlstore.go:4451
// TODO(tjons): refactor this out into some helpers
func validateRegistrationEntry(entry *common.RegistrationEntry) error {
	if entry == nil {
		return newValidationError("invalid request: missing registered entry")
	}

	if len(entry.Selectors) == 0 {
		return newValidationError("invalid registration entry: missing selector list")
	}

	// In case of StoreSvid is set, all entries 'must' be the same type,
	// it is done to avoid users to mix selectors from different platforms in
	// entries with storable SVIDs
	if entry.StoreSvid {
		// Selectors must never be empty
		tpe := entry.Selectors[0].Type
		for _, t := range entry.Selectors {
			if tpe != t.Type {
				return newValidationError("invalid registration entry: selector types must be the same when store SVID is enabled")
			}
		}
	}

	if len(entry.EntryId) > 255 {
		return newValidationError("invalid registration entry: entry ID too long")
	}

	for _, e := range entry.EntryId {
		if !unicode.In(e, validEntryIDChars) {
			return newValidationError("invalid registration entry: entry ID contains invalid characters")
		}
	}

	if len(entry.SpiffeId) == 0 {
		return newValidationError("invalid registration entry: missing SPIFFE ID")
	}

	if entry.X509SvidTtl < 0 {
		return newValidationError("invalid registration entry: X509SvidTtl is not set")
	}

	if entry.JwtSvidTtl < 0 {
		return newValidationError("invalid registration entry: JwtSvidTtl is not set")
	}

	return nil
}

func (p *plugin) CreateOrReturnRegistrationEntry(context.Context, *common.RegistrationEntry) (*common.RegistrationEntry, bool, error) {
	return nil, false, NotImplementedErr
}

func (p *plugin) DeleteRegistrationEntry(ctx context.Context, entryID string) (*common.RegistrationEntry, error) {
	entries, err := fetchRegistrationEntries(p.db.session, []string{entryID})
	if err != nil {
		return nil, newWrappedCassandraError(err)
	}

	if entries[entryID] == nil {
		return nil, status.Error(codes.NotFound, NotFoundErr.Error())
	}

	if err := deleteRegistrationEntry(p.db.session, entries[entryID]); err != nil {
		return nil, newWrappedCassandraError(err)
	}

	return entries[entryID], nil
}

func deleteRegistrationEntry(s *gocql.Session, re *common.RegistrationEntry) error {
	b := s.Batch(gocql.LoggedBatch)

	const deleteEntryRowsQuery = `DELETE FROM registered_entries WHERE entry_id = ?`
	const deleteFederatedBundlesQuery = `DELETE FROM bundles WHERE trust_domain = ? AND federated_entry_id = ?`
	b.Entries = []gocql.BatchEntry{
		{
			Stmt:       deleteEntryRowsQuery,
			Args:       []any{re.EntryId},
			Idempotent: true,
		},
	}

	for _, ftd := range re.FederatesWith {
		b.Entries = append(b.Entries, gocql.BatchEntry{
			Stmt:       deleteFederatedBundlesQuery,
			Args:       []any{ftd, re.EntryId},
			Idempotent: true,
		})
	}

	if err := b.Exec(); err != nil {
		return newWrappedCassandraError(err)
	}

	return nil
}

func (p *plugin) FetchRegistrationEntry(ctx context.Context, entryID string) (*common.RegistrationEntry, error) {
	entries, err := fetchRegistrationEntries(p.db.session, []string{entryID})
	if err != nil {
		return nil, err
	}

	return entries[entryID], nil
}

func fetchRegistrationEntries(session *gocql.Session, entryIDs []string) (map[string]*common.RegistrationEntry, error) {
	fetchRegistrationEntriesQuery := `
		SELECT
			created_at,
			updated_at,
			entry_id,
			spiffe_id,
			parent_id,
			ttl,
			admin,
			downstream,
			expiry,
			revision_number,
			store_svid,
			hint,
			jwt_svid_ttl,
			dns_names,
			federated_trust_domains,
			selector_type,
			selector_value
		FROM registered_entries
	`

	args := []any{}
	cleanedEntryIDs := make([]string, 0, len(entryIDs))
	for _, id := range entryIDs {
		if len(id) > 0 {
			cleanedEntryIDs = append(cleanedEntryIDs, id)
		}
	}
	if len(cleanedEntryIDs) > 0 {
		args = append(args, cleanedEntryIDs)
		fetchRegistrationEntriesQuery += " WHERE entry_id IN ? ALLOW FILTERING"
	}
	// TODO(tjons): I don't think we need to ALLOW FILTERING here because we have an SAI on entry_id
	// but cassandra is rejecting the query during the statement preparation phase unless we include it.
	// Investigate further.

	query := session.Query(fetchRegistrationEntriesQuery, args...)
	query.Consistency(gocql.LocalQuorum)

	iter := query.Iter()
	entryMap := make(map[string]*common.RegistrationEntry, iter.NumRows())
	scanner := iter.Scanner()

	// Since entries can have multiple selectors, we need to aggregate them

	for scanner.Next() {
		var (
			result               = new(common.RegistrationEntry)
			selector             = new(common.Selector)
			createdAt, updatedAt time.Time
		)

		err := scanner.Scan(
			&createdAt,
			&updatedAt,
			&result.EntryId,
			&result.SpiffeId,
			&result.ParentId,
			&result.X509SvidTtl,
			&result.Admin,
			&result.Downstream,
			&result.EntryExpiry,
			&result.RevisionNumber,
			&result.StoreSvid,
			&result.Hint,
			&result.JwtSvidTtl,
			&result.DnsNames,
			&result.FederatesWith,
			&selector.Type,
			&selector.Value,
		)
		if err != nil {
			return nil, newWrappedCassandraError(err)
		}

		if _, ok := entryMap[result.EntryId]; ok {
			entryMap[result.EntryId].Selectors = append(entryMap[result.EntryId].Selectors, selector)
		} else {
			result.CreatedAt = createdAt.Unix()
			result.Selectors = append(result.Selectors, selector)
			entryMap[result.EntryId] = result
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, newWrappedCassandraError(err)
	}

	return entryMap, nil
}

func (p *plugin) FetchRegistrationEntries(ctx context.Context, entryIDs []string) (map[string]*common.RegistrationEntry, error) {
	return fetchRegistrationEntries(p.db.session, entryIDs)
}

func (p *plugin) ListRegistrationEntries(ctx context.Context, req *datastore.ListRegistrationEntriesRequest) (*datastore.ListRegistrationEntriesResponse, error) {
	if req.Pagination != nil && req.Pagination.PageSize == 0 {
		return nil, status.Error(codes.InvalidArgument, "cannot paginate with pagesize = 0")
	}
	if req.BySelectors != nil && len(req.BySelectors.Selectors) == 0 {
		return nil, status.Error(codes.InvalidArgument, "cannot list by empty selector set")
	}

	args := []any{}
	fields := []string{}
	operators := []string{}
	if len(req.ByParentID) > 0 {
		args = append(args, req.ByParentID)
		fields = append(fields, "parent_id")
		operators = append(operators, "=")
	}

	if len(req.BySpiffeID) > 0 {
		args = append(args, req.BySpiffeID)
		fields = append(fields, "spiffe_id")
		operators = append(operators, "=")
	}

	if req.ByDownstream != nil {
		args = append(args, *req.ByDownstream)
		fields = append(fields, "downstream")
		operators = append(operators, "=")
	}

	if req.ByFederatesWith != nil && len(req.ByFederatesWith.TrustDomains) > 0 {
		args = append(args, req.ByFederatesWith.TrustDomains)
		fields = append(fields, "federated_trust_domains")
		operators = append(operators, "CONTAINS")
	}

	if len(req.ByHint) > 0 {
		args = append(args, req.ByHint)
		fields = append(fields, "hint")
		operators = append(operators, "=")
	}

	if req.BySelectors != nil {

	}

	b := strings.Builder{}
	b.WriteString(`
		SELECT 
			created_at,
			updated_at,
			entry_id,
			spiffe_id,
			parent_id,
			ttl,
			admin,
			downstream,
			expiry,
			revision_number,
			store_svid,
			hint,
			jwt_svid_ttl,
			dns_names,
			federated_trust_domains,
			selector_type,
			selector_value
		FROM registered_entries
	`)
	if len(fields) > 0 || (req.Pagination != nil && len(req.Pagination.Token) > 0) {
		b.WriteString(" WHERE ")
	}

	if len(fields) > 0 {
		for i, field := range fields {
			if i > 0 {
				b.WriteString(" AND ")
			}
			b.WriteString(field)
			b.WriteString(" ")
			b.WriteString(operators[i])
			b.WriteString(" ?")
		}
	}

	if req.Pagination != nil && len(req.Pagination.Token) > 0 {
		if len(fields) > 0 {
			b.WriteString(" AND ")
		}
		b.WriteString("TOKEN(entry_id) > TOKEN(?)")
		args = append(args, req.Pagination.Token)
	}

	if req.Pagination != nil && req.Pagination.PageSize > 0 {
		b.WriteString(" LIMIT ")
		b.WriteString(strconv.Itoa(int(req.Pagination.PageSize)))
	}

	b.WriteString(" ALLOW FILTERING")

	query := b.String()
	cqlQuery := p.db.session.Query(query, args...)
	cqlQuery.Consistency(gocql.LocalQuorum)
	iter := cqlQuery.Iter()
	entryMap := make(map[string]*common.RegistrationEntry, iter.NumRows())
	scanner := iter.Scanner()

	// Since entries can have multiple selectors, we need to aggregate them

	for scanner.Next() {
		var (
			result               = new(common.RegistrationEntry)
			selector             = new(common.Selector)
			createdAt, updatedAt time.Time
		)

		err := scanner.Scan(
			&createdAt,
			&updatedAt,
			&result.EntryId,
			&result.SpiffeId,
			&result.ParentId,
			&result.X509SvidTtl,
			&result.Admin,
			&result.Downstream,
			&result.EntryExpiry,
			&result.RevisionNumber,
			&result.StoreSvid,
			&result.Hint,
			&result.JwtSvidTtl,
			&result.DnsNames,
			&result.FederatesWith,
			&selector.Type,
			&selector.Value,
		)
		if err != nil {
			return nil, newWrappedCassandraError(err)
		}

		if _, ok := entryMap[result.EntryId]; ok {
			entryMap[result.EntryId].Selectors = append(entryMap[result.EntryId].Selectors, selector)
		} else {
			result.CreatedAt = createdAt.Unix()
			result.Selectors = append(result.Selectors, selector)
			entryMap[result.EntryId] = result
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, newWrappedCassandraError(err)
	}

	r := &datastore.ListRegistrationEntriesResponse{
		Entries: slices.Collect(maps.Values(entryMap)),
	}

	if req.Pagination != nil {
		r.Pagination = &datastore.Pagination{
			PageSize: req.Pagination.PageSize,
		}
		if len(r.Entries) == int(req.Pagination.PageSize) {
			lastEntry := r.Entries[len(r.Entries)-1]
			r.Pagination.Token = lastEntry.EntryId
		}
	}

	return r, nil
}

func (p *plugin) PruneRegistrationEntries(ctx context.Context, expiresBefore time.Time) error {
	selectPruneQuery := `
		SELECT DISTINCT entry_id, spiffe_id, parent_id FROM registered_entries WHERE expiry IS NOT NULL AND expiry < ?
		`
	query := p.db.session.Query(selectPruneQuery, expiresBefore.Unix())
	query.Consistency(gocql.LocalQuorum)

	iter := query.Iter()

	type entryToPrune struct {
		entryID  string
		spiffeID string
		parentID string
	}

	entries := make(map[string]entryToPrune, iter.NumRows())
	scanner := iter.Scanner()

	for scanner.Next() {
		var entry entryToPrune
		err := scanner.Scan(&entry.entryID, &entry.spiffeID, &entry.parentID)
		if err != nil {
			return newWrappedCassandraError(err)
		}
		entries[entry.entryID] = entry
	}
	if err := iter.Close(); err != nil {
		return newWrappedCassandraError(err)
	}

	delIds := slices.Collect(maps.Keys(entries))
	deletePruneQuery := `DELETE FROM registered_entries WHERE entry_id IN (?)`
	deleteFederatedQuery := `DELETE FROM bundles WHERE federated_entry_id IN (?)`

	b := p.db.session.Batch(gocql.LoggedBatch)

	b.Query(deletePruneQuery, delIds)
	b.Query(deleteFederatedQuery, delIds)

	if err := p.db.session.ExecuteBatch(b); err != nil {
		return newWrappedCassandraError(err)
	}

	// TODO(tjons): handle registration entry events

	for _, entry := range entries {
		p.log.WithFields(logrus.Fields{
			telemetry.SPIFFEID:       entry.spiffeID,
			telemetry.ParentID:       entry.parentID,
			telemetry.RegistrationID: entry.entryID,
		}).Info("Pruned an expired registration")
	}

	return nil
}

func (p *plugin) UpdateRegistrationEntry(context.Context, *common.RegistrationEntry, *common.RegistrationEntryMask) (*common.RegistrationEntry, error) {
	return nil, NotImplementedErr
}
