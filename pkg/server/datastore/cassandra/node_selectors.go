package cassandra

import (
	"context"

	"github.com/spiffe/spire/pkg/server/datastore"
	"github.com/spiffe/spire/proto/spire/common"
)

func (db *plugin) GetNodeSelectors(ctx context.Context, spiffeID string, dataConsistency datastore.DataConsistency) ([]*common.Selector, error) {
	return nil, NotImplementedErr

}

func (db *plugin) ListNodeSelectors(context.Context, *datastore.ListNodeSelectorsRequest) (*datastore.ListNodeSelectorsResponse, error) {
	return nil, NotImplementedErr

}

func (db *plugin) SetNodeSelectors(ctx context.Context, spiffeID string, selectors []*common.Selector) error {
	return NotImplementedErr
}
