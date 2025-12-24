package cassandra

import (
	"context"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	"github.com/spiffe/spire/pkg/server/datastore"
)

func (p *plugin) CreateFederationRelationship(context.Context, *datastore.FederationRelationship) (*datastore.FederationRelationship, error) {
	return nil, NotImplementedErr
}

func (p *plugin) FetchFederationRelationship(context.Context, spiffeid.TrustDomain) (*datastore.FederationRelationship, error) {
	return nil, NotImplementedErr
}

func (p *plugin) ListFederationRelationships(context.Context, *datastore.ListFederationRelationshipsRequest) (*datastore.ListFederationRelationshipsResponse, error) {
	return nil, NotImplementedErr
}

func (p *plugin) DeleteFederationRelationship(context.Context, spiffeid.TrustDomain) error {
	return NotImplementedErr
}

func (p *plugin) UpdateFederationRelationship(context.Context, *datastore.FederationRelationship, *types.FederationRelationshipMask) (*datastore.FederationRelationship, error) {
	return nil, NotImplementedErr

}
