package cassandra

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/spiffe/spire/pkg/server/datastore"
	"github.com/spiffe/spire/proto/spire/common"
)

func (db *plugin) AppendBundle(context.Context, *common.Bundle) (*common.Bundle, error) {
	return nil, NotImplementedErr
}

func (db *plugin) CountBundles(context.Context) (int32, error) {
	return 0, NotImplementedErr
}

func (db *plugin) CreateBundle(context.Context, *common.Bundle) (*common.Bundle, error) {
	return nil, NotImplementedErr
}

func (db *plugin) DeleteBundle(ctx context.Context, trustDomainID string, mode datastore.DeleteMode) error {
	return NotImplementedErr
}

func (db *plugin) FetchBundle(ctx context.Context, trustDomainID string) (*common.Bundle, error) {
	return nil, NotImplementedErr
}

func (db *plugin) ListBundles(context.Context, *datastore.ListBundlesRequest) (*datastore.ListBundlesResponse, error) {
	return nil, NotImplementedErr
}

func (db *plugin) PruneBundle(ctx context.Context, trustDomainID string, expiresBefore time.Time) (changed bool, err error) {
	return false, NotImplementedErr
}

func (db *plugin) SetBundle(context.Context, *common.Bundle) (*common.Bundle, error) {
	return nil, NotImplementedErr
}

func (db *plugin) UpdateBundle(context.Context, *common.Bundle, *common.BundleMask) (*common.Bundle, error) {
	return nil, NotImplementedErr
}

type Bundle struct { // TODO(tjons): next step is to create model objects I think

// copied from sqlstore.go:4538
func bundleToModel(pb *common.Bundle) (*Bundle, error) {
	if pb == nil {
		return nil, newSQLError("missing bundle in request")
	}
	data, err := proto.Marshal(pb)
	if err != nil {
		return nil, newWrappedSQLError(err)
	}
	return &Bundle{
		TrustDomain: pb.TrustDomainId,
		Data:        data,
	}, nil
}
