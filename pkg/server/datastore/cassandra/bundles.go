package cassandra

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gocql/gocql"
	"github.com/gogo/protobuf/proto"
	"github.com/spiffe/spire/pkg/server/datastore"
	"github.com/spiffe/spire/proto/spire/common"
)

func (p *plugin) AppendBundle(context.Context, *common.Bundle) (*common.Bundle, error) {
	return nil, NotImplementedErr
}

func (p *plugin) CountBundles(context.Context) (int32, error) {
	return 0, NotImplementedErr
}

func (p *plugin) CreateBundle(context.Context, *common.Bundle) (*common.Bundle, error) {
	return nil, NotImplementedErr
}

func (p *plugin) DeleteBundle(ctx context.Context, trustDomainID string, mode datastore.DeleteMode) error {
	return NotImplementedErr
}

func (p *plugin) FetchBundle(ctx context.Context, trustDomainID string) (*common.Bundle, error) {
	q := `
	SELECT
		data
	FROM bundles
	WHERE trust_domain = ?
	`
	var data []byte
	query := p.db.session.QueryWithContext(ctx, q, trustDomainID)
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

func (p *plugin) ListBundles(context.Context, *datastore.ListBundlesRequest) (*datastore.ListBundlesResponse, error) {
	return nil, NotImplementedErr
}

func (p *plugin) PruneBundle(ctx context.Context, trustDomainID string, expiresBefore time.Time) (changed bool, err error) {
	return false, NotImplementedErr
}

func (p *plugin) SetBundle(context.Context, *common.Bundle) (*common.Bundle, error) {
	return nil, NotImplementedErr
}

func (p *plugin) UpdateBundle(context.Context, *common.Bundle, *common.BundleMask) (*common.Bundle, error) {
	return nil, NotImplementedErr
}

type Bundle struct { // TODO(tjons): next step is to create model objects I think
	Model

	TrustDomain string // In the gorm implementation this is not null and must be unique
	Data        []byte

	// FederatedEntries []RegisteredEntry
}

// copied from sqlstore.go:4538
func bundleToModel(pb *common.Bundle) (*Bundle, error) {
	// if pb == nil {
	// 	return nil, newSQLError("missing bundle in request")
	// }
	// data, err := proto.Marshal(pb)
	// if err != nil {
	// 	return nil, newWrappedSQLError(err)
	// }
	// return &Bundle{
	// 	TrustDomain: pb.TrustDomainId,
	// 	Data:        data,
	// }, nil

	return nil, nil
}
