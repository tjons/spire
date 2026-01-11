package cassandra

import (
	"context"
	"encoding/base64"
	"fmt"
	"math"
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

type selector struct {
	Type  string
	Value string
}

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
	Selectors             []*selector
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

	if entry.FederatesWith != nil {
		bundles, err := p.ListBundles(ctx, &datastore.ListBundlesRequest{})
		if err != nil {
			return nil, newWrappedCassandraError(err)
		}

		ftds := make(map[string]bool, len(entry.FederatesWith))
		for _, ftd := range entry.FederatesWith {
			ftds[ftd] = false
		}

		for _, b := range bundles.Bundles {
			if _, ok := ftds[b.TrustDomainId]; ok {
				ftds[b.TrustDomainId] = true
			}
		}

		for ftd, found := range ftds {
			if !found {
				return nil, fmt.Errorf("unable to find federated bundle %q", ftd)
			}
		}
	}

	newEntry, err = createRegistrationEntry(ctx, p.db.session, entry)
	if err != nil {
		return nil, err
	}

	err = p.createRegistrationEntryEvent(ctx, &datastore.RegistrationEntryEvent{
		EntryID: newEntry.EntryId,
	})

	return newEntry, err
}

func createRegistrationEntry(ctx context.Context, s *gocql.Session, entry *common.RegistrationEntry) (*common.RegistrationEntry, error) {
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

	b := s.Batch(gocql.LoggedBatch)

	indexes := buildIndexesForRegistrationEntry(entry)

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
			selector_types,
			selector_values,
			index_terms,
			selector_type_value_full,
			federated_trust_domains_full,
			unrolled_selector_type_val,
			unrolled_ftd
		) VALUES (toTimestamp(now()), toTimestamp(now()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	// TODO(tjons): consistency level?
	// is read consistency cheaper than write consistency?

	selectorTypes := make([]string, 0, len(entry.Selectors))
	selectorValues := make([]string, 0, len(entry.Selectors))
	selectorTypeValueFull := make([]string, 0, len(entry.Selectors))

	for _, sl := range entry.Selectors {
		selectorTypes = append(selectorTypes, sl.Type)
		selectorValues = append(selectorValues, sl.Value)
		selectorTypeValueFull = append(selectorTypeValueFull, sl.Type+"|"+sl.Value)
	}

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
		selectorTypes,
		selectorValues,
		indexes,
		selectorTypeValueFull,
		newRegisteredEntry.FederatedTrustDomains,
	}

	b.Entries = []gocql.BatchEntry{
		{
			Stmt: createEntryQuery,
			Args: append(commonVals, "", ""),
		},
	}

	for _, sl := range entry.Selectors {
		selVal := sl.Type + "|" + sl.Value

		b.Entries = append(b.Entries, gocql.BatchEntry{
			Stmt: createEntryQuery,
			Args: append(commonVals, selVal, ""),
		})
	}

	for _, ftd := range entry.FederatesWith {
		b.Entries = append(b.Entries, gocql.BatchEntry{
			Stmt: createEntryQuery,
			Args: append(commonVals, "", ftd),
		})
	}

	if err := b.ExecContext(ctx); err != nil {
		return nil, newWrappedCassandraError(err)
	}

	newRegisteredEntry.CreatedAt = time.Now() // TODO(tjons): this feels hacky?

	for _, sl := range entry.Selectors {
		newRegisteredEntry.Selectors = append(newRegisteredEntry.Selectors, &selector{
			Type:  sl.Type,
			Value: sl.Value,
		})
	}

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
	}

	r.Selectors = make([]*common.Selector, len(re.Selectors))

	for i, s := range re.Selectors {
		r.Selectors[i] = &common.Selector{
			Type:  s.Type,
			Value: s.Value,
		}
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

func (p *plugin) CreateOrReturnRegistrationEntry(ctx context.Context, re *common.RegistrationEntry) (*common.RegistrationEntry, bool, error) {
	if err := validateRegistrationEntry(re); err != nil {
		return nil, false, status.Error(codes.InvalidArgument, err.Error())
	}

	resp, err := p.ListRegistrationEntries(ctx, &datastore.ListRegistrationEntriesRequest{
		ByParentID: re.ParentId,
		BySpiffeID: re.SpiffeId,
		BySelectors: &datastore.BySelectors{
			Match:     datastore.Exact,
			Selectors: re.Selectors,
		},
	})
	if err != nil {
		return nil, false, newWrappedCassandraError(err)
	}

	if len(resp.Entries) > 0 {
		return resp.Entries[0], true, nil
	}

	newEntry, err := p.CreateRegistrationEntry(ctx, re)
	if err != nil {
		return nil, false, err
	}

	if err := p.createRegistrationEntryEvent(ctx, &datastore.RegistrationEntryEvent{
		EntryID: newEntry.EntryId,
	}); err != nil {
		return nil, false, err
	}

	return newEntry, false, nil
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

	if err := p.createRegistrationEntryEvent(ctx, &datastore.RegistrationEntryEvent{
		EntryID: entryID,
	}); err != nil {
		return nil, err
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
			selector_types,
			selector_values
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
	entryMap := make(map[string]*RegistrationEntry, iter.NumRows())
	scanner := iter.Scanner()

	// Since entries can have multiple selectors, we need to aggregate them

	for scanner.Next() {
		var (
			result                        = new(RegistrationEntry)
			selectorTypes, selectorValues []string
		)

		err := scanner.Scan(
			&result.CreatedAt,
			&result.UpdatedAt,
			&result.EntryID,
			&result.SpiffeID,
			&result.ParentID,
			&result.TTL,
			&result.Admin,
			&result.Downstream,
			&result.Expiry,
			&result.RevisionNumber,
			&result.StoreSVID,
			&result.Hint,
			&result.JWTSVIDTTL,
			&result.DNSNames,
			&result.FederatedTrustDomains,
			&selectorTypes,
			&selectorValues,
		)
		if err != nil {
			return nil, newWrappedCassandraError(err)
		}

		for i := range selectorTypes {
			sel := &selector{
				Type:  selectorTypes[i],
				Value: selectorValues[i],
			}
			result.Selectors = append(result.Selectors, sel)
		}

		entryMap[result.EntryID] = result
	}

	if err := scanner.Err(); err != nil {
		return nil, newWrappedCassandraError(err)
	}

	retval := make(map[string]*common.RegistrationEntry, len(entryMap))
	for id, entry := range entryMap {
		retval[id] = registrationEntryModelToProto(entry)
	}

	return retval, nil
}

func (p *plugin) FetchRegistrationEntries(ctx context.Context, entryIDs []string) (map[string]*common.RegistrationEntry, error) {
	return fetchRegistrationEntries(p.db.session, entryIDs)
}

type queryTerm struct {
	field              string
	operator           string
	values             []any
	deepValues         [][]any
	requireDistinct    bool
	includeExtraColumn bool
}

func (p *plugin) ListRegistrationEntries(ctx context.Context, req *datastore.ListRegistrationEntriesRequest) (*datastore.ListRegistrationEntriesResponse, error) {
	if req.Pagination != nil {
		if req.Pagination.PageSize == 0 {
			return nil, status.Error(codes.InvalidArgument, "cannot paginate with pagesize = 0")
		}

		if len(req.Pagination.Token) > 0 {

			pToken, err := base64.URLEncoding.Strict().DecodeString(req.Pagination.Token)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "could not parse token '%s'", req.Pagination.Token)
			}
			req.Pagination.Token = string(pToken) // TODO(tjons): clean this up and avoid the mutation
		}
	}
	if req.BySelectors != nil && len(req.BySelectors.Selectors) == 0 {
		return nil, status.Error(codes.InvalidArgument, "cannot list by empty selector set")
	}

	collapseToPartitionRow := true
	onlyFiltersStaticCols := true
	terms := []queryTerm{}
	if len(req.ByParentID) > 0 {
		terms = append(terms, queryTerm{
			field:    "parent_id",
			operator: "=",
			values:   []any{req.ByParentID},
		})
	}

	if len(req.BySpiffeID) > 0 {
		terms = append(terms, queryTerm{
			field:    "spiffe_id",
			operator: "=",
			values:   []any{req.BySpiffeID},
		})
	}

	if req.ByDownstream != nil {
		terms = append(terms, queryTerm{
			field:    "downstream",
			operator: "=",
			values:   []any{*req.ByDownstream},
		})
	}

	if len(req.ByHint) > 0 {
		terms = append(terms, queryTerm{
			field:    "hint",
			operator: "=",
			values:   []any{req.ByHint},
		})
	}

	if req.ByFederatesWith != nil || req.BySelectors != nil {
		indexes := generateSearchIndexesForRequest(req)
		terms = append(terms, indexes...)
		// TODO(tjons): this has to be temp

		for _, idx := range indexes {
			if idx.operator == "IN" {
				collapseToPartitionRow = false
			}
		}
	}

	addDistinctionColumn := false
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
			selector_types,
			selector_values
		FROM registered_entries  
	`)
	if collapseToPartitionRow {
		// b.WriteString(" unrolled_selector_type_val = '' AND unrolled_ftd = '' ") // no filtering at all, get all entries but limit this to the empty row for paging
		// if len(terms) > 0 {
		// 	b.WriteString(" AND ")
		// }
		// needsDistinct = true
	}

	args := make([]any, 0, len(terms))
	if len(terms) > 0 {
		b.WriteString("WHERE ")

		for i, term := range terms {
			if i > 0 {
				b.WriteString(" AND ")
			}
			b.WriteString(term.field)
			b.WriteString(" ")
			b.WriteString(term.operator)

			if term.operator == "IN" {
				if !addDistinctionColumn {
					addDistinctionColumn = term.includeExtraColumn
					onlyFiltersStaticCols = false
				}

				if term.requireDistinct {
					onlyFiltersStaticCols = true
				}
				// TODO(tjons): the logic in here is actually kinda dangerous
				if len(term.deepValues) > 0 {
					b.WriteString(" (")
					b.WriteString(strings.TrimRight(strings.Repeat(" ?,", len(term.deepValues)), ","))
					b.WriteString(")")
					for _, dv := range term.deepValues {
						args = append(args, dv)
					}
					continue
				}

				b.WriteString(" (")
				b.WriteString(strings.TrimRight(strings.Repeat(" ?,", len(term.values)), ","))
				b.WriteString(")")
			} else {
				b.WriteString(" ?")
			}

			args = append(args, term.values...)
		}
	}

	b.WriteString(" ALLOW FILTERING")

	query := b.String()
	if !addDistinctionColumn {
		query = strings.Replace(query, "updated_at,", "", 1)
	}

	if onlyFiltersStaticCols {
		query = strings.Replace(query, "SELECT", "SELECT DISTINCT", 1)
	}

	cqlQuery := p.db.session.Query(query, args...)
	cqlQuery.Consistency(gocql.LocalQuorum)

	if req.Pagination != nil {
		cqlQuery.PageSize(int(req.Pagination.PageSize))

		if len(req.Pagination.Token) > 0 {
			cqlQuery = cqlQuery.PageState([]byte(req.Pagination.Token))
		} else {
			cqlQuery = cqlQuery.PageState(nil)
		}
	} else {
		cqlQuery.PageSize(100_000_000) // effectively no limit
	}

	iter := cqlQuery.Iter()
	entryMap := make(map[string]*RegistrationEntry, iter.NumRows())
	scanner := iter.Scanner()

	for scanner.Next() {
		var (
			result                        = new(RegistrationEntry)
			selectorTypes, selectorValues []string
			err                           error
		)

		if !addDistinctionColumn {
			err = scanner.Scan(
				&result.CreatedAt,
				&result.EntryID,
				&result.SpiffeID,
				&result.ParentID,
				&result.TTL,
				&result.Admin,
				&result.Downstream,
				&result.Expiry,
				&result.RevisionNumber,
				&result.StoreSVID,
				&result.Hint,
				&result.JWTSVIDTTL,
				&result.DNSNames,
				&result.FederatedTrustDomains,
				&selectorTypes,
				&selectorValues,
			)
		} else {
			err = scanner.Scan(
				&result.CreatedAt,
				&result.UpdatedAt,
				&result.EntryID,
				&result.SpiffeID,
				&result.ParentID,
				&result.TTL,
				&result.Admin,
				&result.Downstream,
				&result.Expiry,
				&result.RevisionNumber,
				&result.StoreSVID,
				&result.Hint,
				&result.JWTSVIDTTL,
				&result.DNSNames,
				&result.FederatedTrustDomains,
				&selectorTypes,
				&selectorValues,
			)
		}
		if err != nil {
			return nil, newWrappedCassandraError(err)
		}

		for i := range selectorTypes {
			selector := &selector{
				Type:  selectorTypes[i],
				Value: selectorValues[i],
			}
			result.Selectors = append(result.Selectors, selector)
		}

		entryMap[result.EntryID] = result
	}

	if err := scanner.Err(); err != nil {
		return nil, newWrappedCassandraError(err)
	}
	pageState := iter.PageState()

	r := &datastore.ListRegistrationEntriesResponse{
		Entries: make([]*common.RegistrationEntry, 0, len(entryMap)),
	}

	for _, entry := range entryMap {
		r.Entries = append(r.Entries, registrationEntryModelToProto(entry))
	}

	if req.Pagination != nil {
		r.Pagination = &datastore.Pagination{
			PageSize: req.Pagination.PageSize,
		}

		// go ahead and "peek"	if there is a next page...
		peeker := p.db.session.Query(query, args...)
		peeker.Consistency(gocql.LocalQuorum)

		peeker.PageState(pageState)
		peeker.PageSize(1)        // I hate all this and i think it would be better if we just dropped the silly next pagination requirement for cassandra
		peekIter := peeker.Iter() // at a minimum, we should feature flag this
		if peekIter.NumRows() > 0 {
			r.Pagination.Token = base64.URLEncoding.Strict().EncodeToString(pageState)
		}
		if err := peekIter.Close(); err != nil {
			return nil, newWrappedCassandraError(err)
		}
	}

	return r, nil
}

func Combinations[T any](els []T) [][]T {
	if len(els) == 0 {
		return [][]T{}
	}

	results := [][]T{}

	count := int(math.Pow(2, float64(len(els))))

	for i := range count {
		subset := []T{}

		for j := range len(els) {
			if (i & (1 << j)) > 0 {
				subset = append(subset, els[j])
			}
		}

		if len(subset) > 0 {
			results = append(results, subset)
		}
	}

	return results
}

func generateSearchIndexesForRequest(req *datastore.ListRegistrationEntriesRequest) []queryTerm {
	var indices []queryTerm
	if req.BySelectors != nil {
		switch req.BySelectors.Match {
		case datastore.Exact:
			indices = append(indices, queryTerm{
				field:           "index_terms",
				operator:        "CONTAINS",
				values:          []any{buildSelectorMatchExactIndex(req.BySelectors.Selectors)},
				requireDistinct: true,
			})
		case datastore.Subset:
			selectors := make([]any, len(req.BySelectors.Selectors))

			for i, sl := range req.BySelectors.Selectors {
				b := strings.Builder{}
				b.WriteString(sl.Type)
				b.WriteString("|")
				b.WriteString(sl.Value)
				selectors[i] = b.String()
			}

			vals := Combinations(selectors)

			indices = append(indices, queryTerm{
				field:           "selector_type_value_full",
				operator:        "IN",
				deepValues:      vals,
				requireDistinct: true,
			})
		case datastore.MatchAny:
			vals := make([]any, len(req.BySelectors.Selectors))
			for i, sl := range req.BySelectors.Selectors {
				b := strings.Builder{}
				b.WriteString(sl.Type)
				b.WriteString("|")
				b.WriteString(sl.Value)
				vals[i] = b.String()
			}

			indices = append(indices, queryTerm{
				field:              "unrolled_selector_type_val",
				operator:           "IN",
				values:             vals,
				includeExtraColumn: true,
			})
		case datastore.Superset:
			b := strings.Builder{}
			b.WriteString(selectorMatchPrefix)
			b.WriteString(matcherSupersetInfix)

			for i, sl := range req.BySelectors.Selectors {
				if i > 0 {
					b.WriteString("__")
				}
				b.WriteString("type_")
				b.WriteString(sl.Type)
				b.WriteString("_value_")
				b.WriteString(sl.Value)
			}

			indices = append(indices, queryTerm{
				field:           "index_terms",
				operator:        "CONTAINS",
				values:          []any{b.String()},
				requireDistinct: true,
			})
		}
	}

	if req.ByFederatesWith != nil {
		switch req.ByFederatesWith.Match {
		case datastore.Exact:
			indices = append(indices, queryTerm{
				field:           "index_terms",
				operator:        "CONTAINS",
				values:          []any{buildFtdExactIndex(req.ByFederatesWith.TrustDomains)},
				requireDistinct: true,
			})
		case datastore.Subset:
			tds := make([]any, len(req.ByFederatesWith.TrustDomains))
			for i, td := range req.ByFederatesWith.TrustDomains {
				tds[i] = td
			}

			vals := Combinations(tds)
			indices = append(indices, queryTerm{
				field:           "federated_trust_domains_full",
				operator:        "IN",
				deepValues:      vals,
				requireDistinct: true,
			})
		case datastore.MatchAny:
			vals := make([]any, len(req.ByFederatesWith.TrustDomains))
			for i, td := range req.ByFederatesWith.TrustDomains {
				vals[i] = td
			}

			indices = append(indices, queryTerm{
				field:              "unrolled_ftd",
				operator:           "IN",
				values:             vals,
				includeExtraColumn: true,
			})
		case datastore.Superset:
			b := strings.Builder{}
			b.WriteString(ftdIndexPrefix)
			b.WriteString(matcherSupersetInfix)
			for i, td := range req.ByFederatesWith.TrustDomains {
				if i > 0 {
					b.WriteString("__")
				}
				b.WriteString("td_")
				b.WriteString(td)
			}

			indices = append(indices, queryTerm{
				field:           "index_terms",
				operator:        "CONTAINS",
				values:          []any{b.String()},
				requireDistinct: true,
			})
		}
	}

	return indices
}

const ftdIndexPrefix = "ftd_"
const multipartIndexPrefix = "mpidx__"

func buildIndexesForRegistrationEntry(re *common.RegistrationEntry) (indexes []string) {
	var sls, tds []string
	if len(re.GetSelectors()) > 0 {
		sls = buildSelectorIndexes(re.GetSelectors())
		indexes = append(indexes, sls...)
	}

	if len(re.GetFederatesWith()) > 0 {
		tds = buildFtdIndexes(re.GetFederatesWith())
		indexes = append(indexes, tds...)
	}

	for _, s := range sls {
		for _, t := range tds {
			b := strings.Builder{}
			b.WriteString(multipartIndexPrefix)
			b.WriteString(s)
			b.WriteString("___")
			b.WriteString(t)

			indexes = append(indexes, b.String())
		}
	}

	return
}

func buildFtdIndexes(trustDomains []string) (indexes []string) {
	indexes = append(indexes, buildFtdAnyMatchIndexes(trustDomains)...)
	indexes = append(indexes, buildFtdExactIndex(trustDomains))
	indexes = append(indexes, buildFtdSupersetMatchIndexes(trustDomains)...)

	return
}

func buildFtdExactIndex(trustDomains []string) string {
	b := strings.Builder{}
	b.WriteString(ftdIndexPrefix)
	b.WriteString(matcherExactInfix)
	for i, td := range trustDomains {
		if i > 0 {
			b.WriteString("__")
		}
		b.WriteString("td_")
		b.WriteString(td)
	}

	return b.String()
}

func buildFtdAnyMatchIndexes(trustDomains []string) []string {
	indexes := make([]string, 0, len(trustDomains))

	for _, td := range trustDomains {
		b := strings.Builder{}
		b.WriteString(ftdIndexPrefix)
		b.WriteString(matcherAnyInfix)
		b.WriteString("td_")
		b.WriteString(td)

		indexes = append(indexes, b.String())
	}

	return indexes
}

func buildFtdSupersetMatchIndexes(trustDomains []string) []string {
	powerset := Combinations(trustDomains)

	indexes := make([]string, 0, len(trustDomains))

	for _, subset := range powerset {
		b := strings.Builder{}
		b.WriteString(ftdIndexPrefix)
		b.WriteString(matcherSupersetInfix)

		for i, sub := range subset {
			if i > 0 {
				b.WriteString("__")
			}
			b.WriteString("td_")
			b.WriteString(sub)
		}

		indexes = append(indexes, b.String())
	}

	return indexes
}

func buildSelectorIndexes(selectors []*common.Selector) (indexes []string) {
	indexes = append(indexes, buildSelectorAnyMatchIndexes(selectors)...)
	indexes = append(indexes, buildSelectorMatchExactIndex(selectors))
	indexes = append(indexes, buildSelectorSupersetMatchIndexes(selectors)...)
	// subset is implemented as "in these, but no others": see filterEntriesBySelectorSet in pkg/server/datastore/sqlstore/sqlstore.go

	return
}

const selectorMatchPrefix = "stv_"
const matcherAnyInfix = "match_any_"
const matcherExactInfix = "match_exact_"
const matcherSupersetInfix = "match_superset_"

func buildSelectorAnyMatchIndexes(selectors []*common.Selector) []string {
	indexes := make([]string, 0, len(selectors))

	for _, s := range selectors {
		b := strings.Builder{}
		b.WriteString(selectorMatchPrefix)
		b.WriteString(matcherAnyInfix)
		b.WriteString("type_")
		b.WriteString(s.Type)
		b.WriteString("_value_")
		b.WriteString(s.Value)

		indexes = append(indexes, b.String())
	}

	return indexes
}

func buildSelectorMatchExactIndex(selectors []*common.Selector) string {
	b := strings.Builder{}
	b.WriteString(selectorMatchPrefix)
	b.WriteString(matcherExactInfix)

	for i, s := range selectors {
		if i > 0 {
			b.WriteString("__")
		}
		b.WriteString("type_")
		b.WriteString(s.Type)
		b.WriteString("_value_")
		b.WriteString(s.Value)
	}

	return b.String()
}

func buildSelectorSupersetMatchIndexes(selectors []*common.Selector) []string {
	powerset := Combinations(selectors)

	indexes := make([]string, 0, len(selectors))

	for _, subset := range powerset {
		b := strings.Builder{}
		b.WriteString(selectorMatchPrefix)
		b.WriteString(matcherSupersetInfix)

		for i, sub := range subset {
			if i > 0 {
				b.WriteString("__")
			}
			b.WriteString("type_")
			b.WriteString(sub.Type)
			b.WriteString("_value_")
			b.WriteString(sub.Value)
		}

		indexes = append(indexes, b.String())
	}

	return indexes
}

func (p *plugin) PruneRegistrationEntries(ctx context.Context, expiresBefore time.Time) error {
	selectPruneQuery := `
		SELECT DISTINCT entry_id, spiffe_id, parent_id, federated_trust_domains FROM registered_entries WHERE expiry < ?
		`
	query := p.db.session.Query(selectPruneQuery, expiresBefore.Unix())
	query.Consistency(gocql.LocalQuorum)

	iter := query.Iter()

	type entryToPrune struct {
		entryID               string
		spiffeID              string
		parentID              string
		federatedTrustDomains []string
	}

	entries := make([]entryToPrune, 0, iter.NumRows())
	scanner := iter.Scanner()

	for scanner.Next() {
		var entry entryToPrune
		err := scanner.Scan(&entry.entryID, &entry.spiffeID, &entry.parentID, &entry.federatedTrustDomains)
		if err != nil {
			return newWrappedCassandraError(err)
		}
		entries = append(entries, entry)
	}
	if err := iter.Close(); err != nil {
		return newWrappedCassandraError(err)
	}

	deletePruneQueryBuilder := strings.Builder{}
	deletePruneQueryBuilder.WriteString(`DELETE FROM registered_entries WHERE entry_id IN (`)

	delIds := make([]any, len(entries))
	b := p.db.session.Batch(gocql.LoggedBatch)

	for i := range entries {
		if i > 0 {
			deletePruneQueryBuilder.WriteString(",")
		}
		deletePruneQueryBuilder.WriteString("?")

		if len(entries[i].federatedTrustDomains) > 0 {
			deleteBundlesArgs := make([]any, 0)
			deleteBundlesQueryBuilder := strings.Builder{}
			deleteBundlesQueryBuilder.WriteString(`DELETE FROM bundles WHERE trust_domain IN (`)
			for _, td := range entries[i].federatedTrustDomains {
				deleteBundlesQueryBuilder.WriteString("?,")
				deleteBundlesArgs = append(deleteBundlesArgs, td)
			}
			deleteBundlesQueryBuilder.WriteString(")")
			deleteBundlesQueryBuilder.WriteString(" AND federated_entry_id = ?")
			deleteBundlesArgs = append(deleteBundlesArgs, entries[i].entryID)

			b.Query(deleteBundlesQueryBuilder.String(), deleteBundlesArgs...)
		}

		delIds[i] = entries[i].entryID
	}
	deletePruneQueryBuilder.WriteString(")")

	b.Query(deletePruneQueryBuilder.String(), delIds...)

	if err := b.Exec(); err != nil {
		return newWrappedCassandraError(err)
	}

	for _, entry := range entries {
		if err := p.createRegistrationEntryEvent(ctx, &datastore.RegistrationEntryEvent{
			EntryID: entry.entryID,
		}); err != nil {
			p.log.WithError(err).WithField(telemetry.RegistrationID, entry.entryID).Error("Failed to create registration entry event for pruned entry")
		}

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
