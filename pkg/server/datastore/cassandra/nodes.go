package cassandra

import (
	"context"
	"time"

	"github.com/spiffe/spire/pkg/server/datastore"
	"github.com/spiffe/spire/proto/spire/common"
)

func (p *plugin) CountAttestedNodes(context.Context, *datastore.CountAttestedNodesRequest) (int32, error) {
	return 0, NotImplementedErr
}
func (p *plugin) CreateAttestedNode(context.Context, *common.AttestedNode) (*common.AttestedNode, error) {
	return nil, NotImplementedErr

}
func (p *plugin) DeleteAttestedNode(ctx context.Context, spiffeID string) (*common.AttestedNode, error) {
	return nil, NotImplementedErr

}
func (p *plugin) FetchAttestedNode(ctx context.Context, spiffeID string) (*common.AttestedNode, error) {
	return nil, NotImplementedErr

}
func (p *plugin) ListAttestedNodes(context.Context, *datastore.ListAttestedNodesRequest) (*datastore.ListAttestedNodesResponse, error) {
	return nil, NotImplementedErr

}
func (p *plugin) UpdateAttestedNode(context.Context, *common.AttestedNode, *common.AttestedNodeMask) (*common.AttestedNode, error) {
	return nil, NotImplementedErr

}
func (p *plugin) PruneAttestedExpiredNodes(ctx context.Context, expiredBefore time.Time, includeNonReattestable bool) error {
	return NotImplementedErr
}
