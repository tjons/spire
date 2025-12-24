package cassandra

import (
	"context"

	"github.com/spiffe/spire/pkg/server/datastore"
	"github.com/spiffe/spire/proto/spire/common"
)

func (p *plugin) GetNodeSelectors(ctx context.Context, spiffeID string, dataConsistency datastore.DataConsistency) ([]*common.Selector, error) {
	return nil, NotImplementedErr

}

func (p *plugin) ListNodeSelectors(context.Context, *datastore.ListNodeSelectorsRequest) (*datastore.ListNodeSelectorsResponse, error) {
	return nil, NotImplementedErr

}

func (p *plugin) SetNodeSelectors(ctx context.Context, spiffeID string, selectors []*common.Selector) error {
	return NotImplementedErr
}
