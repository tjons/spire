package cassandra

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/gocql/gocql"
	"github.com/spiffe/spire/pkg/common/bundleutil"
	"github.com/spiffe/spire/pkg/common/protoutil"
	"github.com/spiffe/spire/pkg/server/datastore"
	"github.com/spiffe/spire/proto/spire/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// TODO(tjons): needs some thought put into consistency for this one...
func (p *plugin) AppendBundle(ctx context.Context, b *common.Bundle) (*common.Bundle, error) {
	existingBundle, err := fetchBundle(p.db.session, b.TrustDomainId)
	if err != nil {
		return nil, err
	}
	if existingBundle == nil {
		createdBundle, err := createBundle(p.db.session, b)
		if err != nil {
			return nil, err
		}
		return createdBundle, nil
	}

	bundle, changed := bundleutil.MergeBundles(existingBundle, b)
	if changed {
		bundle.SequenceNumber++
		newModel, err := bundleToModel(bundle)
		if err != nil {
			return nil, err
		}

		saveQ := `UPDATE bundles SET data = ?, updated_at = toTimestamp(now()) WHERE trust_domain = ?`
		if err = p.db.session.QueryWithContext(
			ctx, saveQ, newModel.Data, newModel.TrustDomain,
		).Exec(); err != nil {
			return nil, newWrappedCassandraError(err)
		}
	}

	return bundle, nil
}

// TODO(tjons): this is going to be... really... expensive.
func (p *plugin) CountBundles(ctx context.Context) (int32, error) {
	countQ := `SELECT COUNT(*) FROM bundles`
	var count int32

	query := p.db.session.QueryWithContext(ctx, countQ)
	if err := query.Scan(&count); err != nil {
		return 0, newWrappedCassandraError(err)
	}

	return count, nil
}

func (p *plugin) CreateBundle(ctx context.Context, newBundle *common.Bundle) (*common.Bundle, error) {
	return createBundle(p.db.session, newBundle)
}

// TODO(tjons): implement the `mode` parameter once I have a better handle on what exactly
// the federated_registration_entries are going to look like in cassandra
func (p *plugin) DeleteBundle(ctx context.Context, trustDomainID string, mode datastore.DeleteMode) error {
	if mode != datastore.Restrict {
		return NotImplementedErr
	}

	exists, err := bundleExistsForTrustDomain(p.db.session, trustDomainID)
	if err != nil {
		return newWrappedCassandraError(err)
	}
	if !exists {
		return status.Error(codes.NotFound, NotFoundErr.Error())
	}

	deleteQ := `DELETE FROM bundles WHERE trust_domain = ?`
	query := p.db.session.Query(deleteQ, trustDomainID)
	if err = query.Exec(); err != nil {
		return newWrappedCassandraError(err)
	}

	return nil
}

func (p *plugin) FetchBundle(ctx context.Context, trustDomainID string) (*common.Bundle, error) {
	return fetchBundle(p.db.session, trustDomainID)
}

// why duplicate with the public wrapper methods? originally, I _hated_ this from the
// sqlstore implementation, but now I don't think it's so bad, because parameter tuning
// etc (contexts, timeouts, consistency levels) will be easier to accomplish outside the
// interface
func fetchBundle(s *gocql.Session, trustDomainID string) (*common.Bundle, error) {
	q := `
	SELECT
		data
	FROM bundles
	WHERE trust_domain = ?
	`
	var data []byte
	query := s.Query(q, trustDomainID)
	if err := query.Scan(&data); err != nil {
		if errors.Is(err, gocql.ErrNotFound) {
			// The existing datastore implementation does not return an error when no results are found
			return nil, nil
		}

		return nil, fmt.Errorf("Error scanning from bundles: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("No bundle found with trust domain ID %s", trustDomainID)
	}

	return dataToBundle(data)
}

func dataToBundle(data []byte) (*common.Bundle, error) {
	bundle := new(common.Bundle)
	if err := proto.Unmarshal(data, bundle); err != nil {
		return nil, err
	}

	return bundle, nil
}

func (p *plugin) ListBundles(ctx context.Context, req *datastore.ListBundlesRequest) (*datastore.ListBundlesResponse, error) {
	if req.Pagination != nil && req.Pagination.PageSize == 0 {
		return nil, status.Error(codes.InvalidArgument, "cannot paginate with pagesize = 0")
	}

	// use `token(trust_domain) > x for pagination`
	var (
		q      = `SELECT DISTINCT trust_domain, data FROM bundles`
		params = make([]any, 0)
	)
	if req.Pagination != nil {
		if len(req.Pagination.Token) > 0 {
			token, err := base64.RawStdEncoding.DecodeString(req.Pagination.Token)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, "pagination token could not be decoded")
			}

			q += " WHERE TOKEN(trust_domain) > ?"
			params = append(params, token)
		}
		q += " LIMIT ?"
		params = append(params, req.Pagination.PageSize)
	}

	resp := &datastore.ListBundlesResponse{
		Pagination: req.Pagination,
	}
	query := p.db.session.Query(q, params...)
	scanner := query.Iter().Scanner()
	for scanner.Next() {
		b := Bundle{}
		if err := scanner.Scan(&b.TrustDomain, &b.Data); err != nil {
			return nil, newWrappedCassandraError(err)
		}
		if req.Pagination != nil {
			req.Pagination.Token = base64.RawStdEncoding.EncodeToString([]byte(b.TrustDomain))
		}

		cb, err := dataToBundle(b.Data)
		if err != nil {
			return nil, newWrappedCassandraError(err)
		}

		resp.Bundles = append(resp.Bundles, cb)
	}

	return resp, nil
}

func (p *plugin) PruneBundle(ctx context.Context, trustDomainID string, expiresBefore time.Time) (changed bool, err error) {
	// This is vulnerable to a race without tuning the consistency and versioning the data.
	currentBundle, err := fetchBundle(p.db.session, trustDomainID)
	if err != nil {
		return false, fmt.Errorf("unable to fetch current bundle: %w", err)
	}

	if currentBundle == nil {
		return false, nil
	}

	newBundle, changed, err := bundleutil.PruneBundle(currentBundle, expiresBefore, p.log)
	if err != nil {
		return false, fmt.Errorf("prune failed: %w", err)
	}

	if changed {
		newBundle.SequenceNumber = currentBundle.SequenceNumber + 1
		if _, err = updateBundle(p.db.session, newBundle, nil); err != nil {
			return false, fmt.Errorf("unable to write new bundle: %w", err)
		}
	}

	return changed, nil
}

func bundleExistsForTrustDomain(s *gocql.Session, trustDomainID string) (bool, error) {
	var count int
	existsQ := `
	SELECT COUNT(*)
	FROM bundles
	WHERE trust_domain = ?`

	query := s.Query(existsQ, trustDomainID)
	if err := query.Scan(&count); err != nil {
		if errors.Is(err, gocql.ErrNotFound) {
			return false, nil
		}

		return false, err
	}

	return count > 0, nil
}

func (p *plugin) SetBundle(ctx context.Context, b *common.Bundle) (*common.Bundle, error) {
	exists, err := bundleExistsForTrustDomain(p.db.session, b.TrustDomainId)
	if err != nil {
		return nil, newWrappedCassandraError(err)
	}

	if !exists {
		bundle, err := createBundle(p.db.session, b)
		if err != nil {
			return nil, err
		}
		return bundle, nil
	}

	bundle, err := updateBundle(p.db.session, b, nil)
	if err != nil {
		return nil, err
	}
	return bundle, nil
}

func createBundle(s *gocql.Session, newBundle *common.Bundle) (*common.Bundle, error) {
	model, err := bundleToModel(newBundle)
	if err != nil {
		return nil, err
	}

	// The Bundle will always have a row set with an empty federated_entry_spiffe_id.
	// This allows federation relationships to come and go without impacting the bundle data itself,
	// and simplifies query patterns.
	createQ := `
	INSERT INTO bundles (created_at, updated_at, trust_domain, data, federated_entry_spiffe_id)
	VALUES (toTimestamp(now()), toTimestamp(now()), ?, ?, '') IF NOT EXISTS
	`
	query := s.Query(createQ, model.TrustDomain, model.Data)
	// TODO(tjons): the use of Quorum consistency here is probably something we
	// want to think about a bit more, but for a POC it's sufficient.
	// Consider whether we'd be better off using the versioned compare and set approach.
	query.Consistency(gocql.Quorum)

	var (
		queryResult = make(map[string]any)
		executed    bool
	)

	if executed, err = query.MapScanCAS(queryResult); err != nil {
		return nil, newWrappedCassandraError(err)
	}

	if !executed {
		return nil, status.Error(codes.AlreadyExists, "bundle with that trust domain ID already exists")
	}

	return newBundle, nil
}

func updateBundle(s *gocql.Session, newBundle *common.Bundle, mask *common.BundleMask) (*common.Bundle, error) {
	newModel, err := bundleToModel(newBundle)
	if err != nil {
		return nil, err
	}

	existingModel := &Bundle{}
	readQ := `
	SELECT DISTINCT created_at, updated_at, trust_domain, data
	FROM bundles WHERE trust_domain = ?
	`

	query := s.Query(readQ, newModel.TrustDomain)
	// TODO(tjons): consider the appropriate quorum to use here
	if err := query.Scan(
		&existingModel.CreatedAt,
		&existingModel.UpdatedAt,
		&existingModel.TrustDomain,
		&existingModel.Data,
	); err != nil {
		if errors.Is(err, gocql.ErrNotFound) {
			return nil, status.Error(codes.NotFound, NotFoundErr.Error())
		}

		return nil, newCassandraError("could not read existing bundle: %s", err.Error())
	}

	existingModel.Data, newBundle, err = applyBundleMask(existingModel, newBundle, mask)
	if err != nil {
		return nil, newWrappedCassandraError(err)
	}

	updateQ := `
	UPDATE bundles 
	SET updated_at = toTimestamp(now()),
		data = ?
	WHERE trust_domain = ?
	`
	query = s.Query(updateQ, newModel.Data, newModel.TrustDomain)
	if err = query.Exec(); err != nil {
		return nil, newWrappedCassandraError(err)
	}

	return newBundle, nil
}

// Copied ~nearly verbatim from pkg/server/datastore/sqlstore/sqlstore.go:1158
func applyBundleMask(model *Bundle, newBundle *common.Bundle, inputMask *common.BundleMask) ([]byte, *common.Bundle, error) {
	bundle, err := dataToBundle(model.Data)
	if err != nil {
		return nil, nil, err
	}

	if inputMask == nil {
		inputMask = protoutil.AllTrueCommonBundleMask
	}

	if inputMask.RefreshHint {
		bundle.RefreshHint = newBundle.RefreshHint
	}

	if inputMask.RootCas {
		bundle.RootCas = newBundle.RootCas
	}

	if inputMask.JwtSigningKeys {
		bundle.JwtSigningKeys = newBundle.JwtSigningKeys
	}

	if inputMask.SequenceNumber {
		bundle.SequenceNumber = newBundle.SequenceNumber
	}

	newModel, err := bundleToModel(bundle)
	if err != nil {
		return nil, nil, err
	}

	return newModel.Data, bundle, nil
}

func (p *plugin) UpdateBundle(ctx context.Context, b *common.Bundle, mask *common.BundleMask) (*common.Bundle, error) {
	return updateBundle(p.db.session, b, mask)
}

type Bundle struct { // TODO(tjons): next step is to create model objects I think
	Model

	TrustDomain string // In the gorm implementation this is not null and must be unique
	Data        []byte

	// FederatedEntries []RegisteredEntry
}

// copied from sqlstore.go:4538
func bundleToModel(pb *common.Bundle) (*Bundle, error) {
	if pb == nil {
		return nil, newCassandraError("missing bundle in request")
	}
	data, err := proto.Marshal(pb)
	if err != nil {
		return nil, newWrappedCassandraError(err)
	}
	return &Bundle{
		TrustDomain: pb.TrustDomainId,
		Data:        data,
	}, nil
}
