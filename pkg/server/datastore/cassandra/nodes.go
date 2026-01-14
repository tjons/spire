package cassandra

import (
	"context"
	"strings"
	"time"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
	"github.com/spiffe/spire/pkg/server/datastore"
	"github.com/spiffe/spire/pkg/server/datastore/cassandra/qb"
	"github.com/spiffe/spire/proto/spire/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AttestedNode struct {
	Model

	SpiffeID        string
	DataType        string
	SerialNumber    string
	ExpiresAt       time.Time
	NewSerialNumber string
	NewExpiresAt    *time.Time
	CanReattest     bool
	Selectors       []*selector
}

func (p *plugin) CountAttestedNodes(ctx context.Context, req *datastore.CountAttestedNodesRequest) (int32, error) {
	return 0, NotImplementedErr
}

func (p *plugin) CreateAttestedNode(ctx context.Context, node *common.AttestedNode) (*common.AttestedNode, error) {
	if node == nil {
		return nil, newCassandraError("invalid request: missing attested node")
	}

	newAttestedNode := &AttestedNode{
		SpiffeID:        node.SpiffeId,
		DataType:        node.AttestationDataType,
		SerialNumber:    node.CertSerialNumber,
		ExpiresAt:       time.Unix(node.CertNotAfter, 0),
		NewSerialNumber: node.NewCertSerialNumber,
		CanReattest:     node.CanReattest,
	}

	if node.NewCertNotAfter != 0 {
		newAttestedNode.NewExpiresAt = &time.Time{}
		*newAttestedNode.NewExpiresAt = time.Unix(node.NewCertNotAfter, 0)
	}

	for _, sel := range node.Selectors {
		newAttestedNode.Selectors = append(newAttestedNode.Selectors, &selector{
			Type:  sel.Type,
			Value: sel.Value,
		})
	}

	created, err := p.createAttestedNode(ctx, newAttestedNode)
	if err != nil {
		return nil, err
	}

	err = p.createAttestedNodeEvent(ctx, &datastore.AttestedNodeEvent{
		SpiffeID: node.SpiffeId,
	})

	return modelToAttestedNode(created), err
}

func (p *plugin) createAttestedNode(ctx context.Context, model *AttestedNode) (*AttestedNode, error) {
	createAttestedNodeQuery := `
		INSERT INTO attested_node_entries (
			created_at,
			updated_at,
			spiffe_id,
			data_type,
			serial_number,
			expires_at,
			new_serial_number,
			new_expires_at,
			can_reattest,
			selector_type_value,
			selector_type_value_full
		) VALUES (toTimestamp(now()), toTimestamp(now()), ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var selectorTypeValue []string
	for _, sel := range model.Selectors {
		selectorTypeValue = append(selectorTypeValue, sel.Type+"|"+sel.Value)
	}

	b := p.db.session.Batch(gocql.LoggedBatch)
	b.Query(
		createAttestedNodeQuery,
		model.SpiffeID,
		model.DataType,
		model.SerialNumber,
		model.ExpiresAt,
		model.NewSerialNumber,
		model.NewExpiresAt,
		model.CanReattest,
		"",
		selectorTypeValue,
	)

	for _, stv := range selectorTypeValue {
		b.Query(createAttestedNodeQuery,
			model.SpiffeID,
			model.DataType,
			model.SerialNumber,
			model.ExpiresAt,
			model.NewSerialNumber,
			model.NewExpiresAt,
			model.CanReattest,
			stv,
			selectorTypeValue,
		)
	}

	if err := b.ExecContext(ctx); err != nil {
		return nil, newWrappedCassandraError(err)
	}

	model.CreatedAt = time.Now()
	model.UpdatedAt = model.CreatedAt

	err := p.createAttestedNodeEvent(ctx, &datastore.AttestedNodeEvent{
		SpiffeID: model.SpiffeID,
	})

	return model, err
}

func modelToAttestedNode(model *AttestedNode) *common.AttestedNode {
	if model == nil {
		return nil
	}

	node := &common.AttestedNode{
		SpiffeId:            model.SpiffeID,
		AttestationDataType: model.DataType,
		CertSerialNumber:    model.SerialNumber,
		CertNotAfter:        model.ExpiresAt.Unix(),
		NewCertSerialNumber: model.NewSerialNumber,
		CanReattest:         model.CanReattest,
	}

	if model.NewExpiresAt != nil {
		node.NewCertNotAfter = model.NewExpiresAt.Unix()
	}

	return node
}

func (p *plugin) DeleteAttestedNode(ctx context.Context, spiffeID string) (*common.AttestedNode, error) {
	attestedNode, err := p.FetchAttestedNode(ctx, spiffeID)
	if err != nil {
		return nil, err
	}
	if attestedNode == nil {
		return nil, status.Error(codes.NotFound, NotFoundErr.Error())
	}

	query := qb.NewDelete().
		From("attested_node_entries").
		Where("spiffe_id", qb.Eq, spiffeID)
	q, _ := query.Build()
	if err := p.db.session.Query(q, spiffeID).ExecContext(ctx); err != nil {
		return nil, newWrappedCassandraError(err)
	}

	err = p.createAttestedNodeEvent(ctx, &datastore.AttestedNodeEvent{
		SpiffeID: spiffeID,
	})

	return attestedNode, err
}

func (p *plugin) FetchAttestedNode(ctx context.Context, spiffeID string) (*common.AttestedNode, error) {
	q := qb.NewSelect().
		Column("spiffe_id").
		Column("data_type").
		Column("serial_number").
		Column("expires_at").
		Column("new_serial_number").
		Column("new_expires_at").
		Column("can_reattest").
		Column("selector_type_value_full").
		From("attested_node_entries").
		Where("spiffe_id", qb.Eq, spiffeID).
		Limit(1)
	query, _ := q.Build()

	var (
		model                 AttestedNode
		selectorTypeValueFull []string
	)
	if err := p.db.session.Query(query, spiffeID).ScanContext(ctx,
		&model.SpiffeID,
		&model.DataType,
		&model.SerialNumber,
		&model.ExpiresAt,
		&model.NewSerialNumber,
		&model.NewExpiresAt,
		&model.CanReattest,
		&selectorTypeValueFull,
	); err != nil {
		if err == gocql.ErrNotFound {
			return nil, nil
		}
		return nil, newWrappedCassandraError(err)
	}

	for _, stv := range selectorTypeValueFull {
		var sel selector
		sel.Type, sel.Value, _ = strings.Cut(stv, "|")
		model.Selectors = append(model.Selectors, &sel)
	}

	return modelToAttestedNode(&model), nil
}

func (p *plugin) ListAttestedNodes(ctx context.Context, req *datastore.ListAttestedNodesRequest) (*datastore.ListAttestedNodesResponse, error) {
	return nil, NotImplementedErr
}

func (p *plugin) UpdateAttestedNode(ctx context.Context, node *common.AttestedNode, mask *common.AttestedNodeMask) (*common.AttestedNode, error) {
	return nil, NotImplementedErr
}

func (p *plugin) PruneAttestedExpiredNodes(ctx context.Context, expiredBefore time.Time, includeNonReattestable bool) error {
	return NotImplementedErr
}

func (p *plugin) GetNodeSelectors(ctx context.Context, spiffeID string, dataConsistency datastore.DataConsistency) ([]*common.Selector, error) {
	q := qb.NewSelect().
		Column("selector_type_value_full").
		From("attested_node_entries").
		Where("spiffe_id", qb.Eq, spiffeID).
		Limit(1)
	query, _ := q.Build()

	var selectorTypeValueFull []string
	if err := p.db.session.Query(query, spiffeID).ScanContext(ctx, &selectorTypeValueFull); err != nil {
		if err == gocql.ErrNotFound {
			return nil, status.Error(codes.NotFound, NotFoundErr.Error())
		}
		return nil, newWrappedCassandraError(err)
	}

	selectors := make([]*common.Selector, len(selectorTypeValueFull))
	for i, stv := range selectorTypeValueFull {
		selType, selValue, _ := strings.Cut(stv, "|")
		selectors[i] = &common.Selector{
			Type:  selType,
			Value: selValue,
		}
	}

	return selectors, nil
}

func (p *plugin) ListNodeSelectors(ctx context.Context, req *datastore.ListNodeSelectorsRequest) (*datastore.ListNodeSelectorsResponse, error) {
	q := qb.NewSelect().
		Distinct().
		Column("spiffe_id").
		Column("selector_type_value_full").
		From("attested_node_entries").
		AllowFiltering()

	if req.ValidAt.Unix() != 0 {
		q.Where("expires_at", qb.Gt, req.ValidAt)
	}

	query, _ := q.Build()

	iter := p.db.session.Query(query).Iter()
	scanner := iter.Scanner()
	resp := &datastore.ListNodeSelectorsResponse{
		Selectors: make(map[string][]*common.Selector, iter.NumRows()),
	}

	for scanner.Next() {
		var (
			spiffeID string
			stvList  []string
		)
		if err := scanner.Scan(&spiffeID, &stvList); err != nil {
			return nil, newWrappedCassandraError(err)
		}

		for _, stv := range stvList {
			selType, selValue, _ := strings.Cut(stv, "|")
			resp.Selectors[spiffeID] = append(resp.Selectors[spiffeID], &common.Selector{
				Type:  selType,
				Value: selValue,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, newWrappedCassandraError(err)
	}

	return resp, nil
}

func (p *plugin) SetNodeSelectors(ctx context.Context, spiffeID string, selectors []*common.Selector) error {
	existingSelectors, err := p.GetNodeSelectors(ctx, spiffeID, datastore.RequireCurrent)
	if err != nil {
		return err
	}

	selectorsToDelete := make(map[string]struct{}, len(existingSelectors))
	selectorsToInsert := make(map[string]struct{}, len(selectors))
	for _, sel := range existingSelectors {
		key := sel.Type + "|" + sel.Value
		selectorsToDelete[key] = struct{}{}
	}

	for _, sel := range selectors {
		key := sel.Type + "|" + sel.Value
		delete(selectorsToDelete, key)
	}

	return p.createAttestedNodeEvent(ctx, &datastore.AttestedNodeEvent{
		SpiffeID: spiffeID,
	})
}
