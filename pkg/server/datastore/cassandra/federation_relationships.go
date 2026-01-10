package cassandra

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	"github.com/spiffe/spire/pkg/common/protoutil"
	"github.com/spiffe/spire/pkg/server/datastore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (p *plugin) CreateFederationRelationship(ctx context.Context, fr *datastore.FederationRelationship) (*datastore.FederationRelationship, error) {
	if err := validateFederationRelationship(fr, protoutil.AllTrueFederationRelationshipMask); err != nil {
		return nil, err
	}

	createQ := `
		INSERT INTO federated_trust_domains (
			created_at,
			updated_at,
			trust_domain,
			bundle_endpoint_url,
			bundle_endpoint_profile,
			endpoint_spiffe_id
		) VALUES (toTimestamp(now()), toTimestamp(now()), ?, ?, ?, ?) IF NOT EXISTS
	`

	if fr.TrustDomainBundle != nil {
		_, err := p.SetBundle(ctx, fr.TrustDomainBundle)
		if err != nil {
			return nil, fmt.Errorf("unable to set bundle: %w", err)
		}
	}

	var esi string
	if fr.BundleEndpointProfile == datastore.BundleEndpointSPIFFE {
		esi = fr.EndpointSPIFFEID.String()
	}

	applied, err := p.db.session.Query(createQ,
		fr.TrustDomain.String(),
		fr.BundleEndpointURL.String(),
		fr.BundleEndpointProfile,
		esi,
	).ScanCAS()
	if err != nil {
		return nil, newWrappedCassandraError(fmt.Errorf("unable to create federation relationship: %w", err))
	}
	if !applied {
		return nil, status.Error(codes.AlreadyExists, "federation relationship already exists")
	}

	return fr, nil
}

type federatedTrustDomainRecord struct {
	Model

	TrustDomain           string
	BundleEndpointURL     string
	BundleEndpointProfile string
	EndpointSPIFFEID      string
}

func (p *plugin) FetchFederationRelationship(ctx context.Context, td spiffeid.TrustDomain) (*datastore.FederationRelationship, error) {
	if td.IsZero() {
		return nil, status.Error(codes.InvalidArgument, "trust domain is required")
	}

	record := new(federatedTrustDomainRecord)
	fetchQ := `
		SELECT
			trust_domain,
			bundle_endpoint_url,
			bundle_endpoint_profile,
			endpoint_spiffe_id
		FROM federated_trust_domains
		WHERE trust_domain = ?
	`

	if err := p.db.session.Query(fetchQ, td.String()).Scan(
		&record.TrustDomain,
		&record.BundleEndpointURL,
		&record.BundleEndpointProfile,
		&record.EndpointSPIFFEID,
	); err != nil {
		if errors.Is(err, gocql.ErrNotFound) {
			return nil, nil
		}
		return nil, newWrappedCassandraError(fmt.Errorf("unable to fetch federation relationship: %w", err))
	}

	return modelToFederationRelationship(p.db.session, record)
}

func modelToFederationRelationship(s *gocql.Session, model *federatedTrustDomainRecord) (*datastore.FederationRelationship, error) {
	bundleEndpointURL, err := url.Parse(model.BundleEndpointURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse URL: %w", err)
	}

	td, err := spiffeid.TrustDomainFromString(model.TrustDomain)
	if err != nil {
		return nil, newWrappedCassandraError(err)
	}

	fr := &datastore.FederationRelationship{
		TrustDomain:           td,
		BundleEndpointURL:     bundleEndpointURL,
		BundleEndpointProfile: datastore.BundleEndpointType(model.BundleEndpointProfile),
	}

	switch fr.BundleEndpointProfile {
	case datastore.BundleEndpointWeb:
	case datastore.BundleEndpointSPIFFE:
		endpointSPIFFEID, err := spiffeid.FromString(model.EndpointSPIFFEID)
		if err != nil {
			return nil, fmt.Errorf("unable to parse bundle endpoint SPIFFE ID: %w", err)
		}
		fr.EndpointSPIFFEID = endpointSPIFFEID
	default:
		return nil, fmt.Errorf("unknown bundle endpoint profile type: %q", model.BundleEndpointProfile)
	}

	trustDomainBundle, err := fetchBundle(s, td.IDString())
	if err != nil && !errors.Is(err, gocql.ErrNotFound) {
		return nil, fmt.Errorf("unable to fetch bundle: %w", err)
	}

	fr.TrustDomainBundle = trustDomainBundle

	return fr, nil
}

func (p *plugin) ListFederationRelationships(ctx context.Context, req *datastore.ListFederationRelationshipsRequest) (*datastore.ListFederationRelationshipsResponse, error) {
	if req.Pagination != nil && req.Pagination.PageSize == 0 {
		return nil, status.Error(codes.InvalidArgument, "cannot paginate with pagesize = 0")
	}

	listQ := `SELECT
			trust_domain,
			bundle_endpoint_url,
			bundle_endpoint_profile,
			endpoint_spiffe_id
		FROM federated_trust_domains
		ALLOW FILTERING
	`
	query := p.db.session.Query(listQ)

	if req.Pagination != nil {
		query = query.PageSize(int(req.Pagination.PageSize))

		if req.Pagination.Token != "" {
			t, err := base64.StdEncoding.DecodeString(req.Pagination.Token)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "could not parse token '%s'", req.Pagination.Token)
			}

			query = query.PageState(t)
		}
	}

	resp := &datastore.ListFederationRelationshipsResponse{
		FederationRelationships: []*datastore.FederationRelationship{},
	}
	iter := query.Iter()
	nextPageState := iter.PageState()
	scanner := iter.Scanner()
	for scanner.Next() {
		record := new(federatedTrustDomainRecord)
		if err := scanner.Scan(
			&record.TrustDomain,
			&record.BundleEndpointURL,
			&record.BundleEndpointProfile,
			&record.EndpointSPIFFEID,
		); err != nil {
			return nil, newWrappedCassandraError(fmt.Errorf("unable to scan federation relationship: %w", err))
		}

		fr, err := modelToFederationRelationship(p.db.session, record)
		if err != nil {
			return nil, fmt.Errorf("unable to convert federation relationship: %w", err)
		}

		resp.FederationRelationships = append(resp.FederationRelationships, fr)
	}
	if err := scanner.Err(); err != nil {
		return nil, newWrappedCassandraError(fmt.Errorf("unable to list federation relationships: %w", err))
	}

	if req.Pagination != nil {
		resp.Pagination = &datastore.Pagination{
			PageSize: req.Pagination.PageSize,
		}

		if len(nextPageState) > 0 {
			resp.Pagination.Token = base64.StdEncoding.EncodeToString(nextPageState)
		}
	}

	return resp, nil
}

func (p *plugin) DeleteFederationRelationship(ctx context.Context, td spiffeid.TrustDomain) error {
	if td.IsZero() {
		return status.Error(codes.InvalidArgument, "trust domain is required")
	}

	deleteQ := `DELETE FROM federated_trust_domains WHERE trust_domain = ? IF EXISTS`
	applied, err := p.db.session.Query(deleteQ, td.String()).ScanCAS()
	if err != nil {
		return newWrappedCassandraError(fmt.Errorf("unable to delete federation relationship: %w", err))
	}
	if !applied {
		return status.Error(codes.NotFound, NotFoundErr.Error())
	}

	return nil
}

func (p *plugin) UpdateFederationRelationship(ctx context.Context, fr *datastore.FederationRelationship, mask *types.FederationRelationshipMask) (*datastore.FederationRelationship, error) {
	if err := validateFederationRelationship(fr, mask); err != nil {
		return nil, err
	}

	// TODO(tjons): consistency!!
	args := []any{}
	fields := []string{}

	if mask.BundleEndpointUrl {
		fields = append(fields, "bundle_endpoint_url = ?")
		args = append(args, fr.BundleEndpointURL.String())
	}

	if mask.BundleEndpointProfile {
		fields = append(fields, "bundle_endpoint_profile = ?")
		args = append(args, fr.BundleEndpointProfile)

		if fr.BundleEndpointProfile == datastore.BundleEndpointSPIFFE {
			fields = append(fields, "endpoint_spiffe_id = ?")
			args = append(args, fr.EndpointSPIFFEID.String())
		}
	}

	if mask.TrustDomainBundle && fr.TrustDomainBundle != nil {
		_, err := p.SetBundle(ctx, fr.TrustDomainBundle) // TODO(tjons): handle this in a batch
		if err != nil {
			return nil, fmt.Errorf("unable to set bundle: %w", err)
		}
	}

	updateQ := strings.Builder{}
	updateQ.WriteString("UPDATE federated_trust_domains SET updated_at = toTimestamp(now())")
	for _, field := range fields {
		updateQ.WriteString(", ")
		updateQ.WriteString(field)
	}
	updateQ.WriteString(" WHERE trust_domain = ? IF EXISTS")
	args = append(args, fr.TrustDomain.String())

	applied, err := p.db.session.Query(updateQ.String(), args...).ScanCAS()
	if err != nil {
		return nil, newWrappedCassandraError(fmt.Errorf("unable to update federation relationship: %w", err))
	}
	if !applied {
		return nil, status.Error(codes.NotFound, "unable to fetch federation relationship: record not found")
	}

	return p.FetchFederationRelationship(ctx, fr.TrustDomain)
}

// Copied verbatim from pkg/server/datastore/sqlstore/sqlstore.go:4416
func validateFederationRelationship(fr *datastore.FederationRelationship, mask *types.FederationRelationshipMask) error {
	if fr == nil {
		return status.Error(codes.InvalidArgument, "federation relationship is nil")
	}

	if fr.TrustDomain.IsZero() {
		return status.Error(codes.InvalidArgument, "trust domain is required")
	}

	if mask.BundleEndpointUrl && fr.BundleEndpointURL == nil {
		return status.Error(codes.InvalidArgument, "bundle endpoint URL is required")
	}

	if mask.BundleEndpointProfile {
		switch fr.BundleEndpointProfile {
		case datastore.BundleEndpointWeb:
		case datastore.BundleEndpointSPIFFE:
			if fr.EndpointSPIFFEID.IsZero() {
				return status.Error(codes.InvalidArgument, "bundle endpoint SPIFFE ID is required")
			}
		default:
			return status.Errorf(codes.InvalidArgument, "unknown bundle endpoint profile type: %q", fr.BundleEndpointProfile)
		}
	}

	return nil
}
