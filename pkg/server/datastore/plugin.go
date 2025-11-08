package datastore

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	"github.com/spiffe/spire/pkg/common/telemetry"
	"github.com/spiffe/spire/proto/spire/common"
)

type Plugin struct {
	mu        sync.Mutex
	dbPlugins []struct {
		store DataStore
		name  string
	}
	log logrus.FieldLogger
	// sqlConfig
}

func NewChainedDatastore(log logrus.FieldLogger) *Plugin {
	return &Plugin{
		mu: sync.Mutex{},
		dbPlugins: []struct {
			store DataStore
			name  string
		}{},
		log: log.WithField(telemetry.SubsystemName, "chained_datastore"),
	}
}

func (p *Plugin) AddDataStore(ds DataStore, name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dbPlugins = append(p.dbPlugins, struct {
		store DataStore
		name  string
	}{store: ds, name: name})
}

func (p *Plugin) AppendBundle(ctx context.Context, bundle *common.Bundle) (*common.Bundle, error) {
	var (
		retval *common.Bundle
		err    error
	)

	for _, plugin := range p.dbPlugins {
		if retval, err = plugin.store.AppendBundle(ctx, bundle); err != nil {
			return nil, err
		}
	}

	return retval, nil
}

func (p *Plugin) CountBundles(ctx context.Context) (int32, error) {
	var (
		count int32
		err   error
	)

	for _, plugin := range p.dbPlugins {
		if count, err = plugin.store.CountBundles(ctx); err != nil {
			return 0, err
		}
	}

	return count, nil
}

func (p *Plugin) CreateBundle(ctx context.Context, bundle *common.Bundle) (*common.Bundle, error) {
	var (
		retval *common.Bundle
		err    error
	)

	for _, plugin := range p.dbPlugins {
		if retval, err = plugin.store.CreateBundle(ctx, bundle); err != nil {
			return nil, err
		}
	}

	return retval, nil
}
func (p *Plugin) DeleteBundle(ctx context.Context, trustDomainID string, mode DeleteMode) error {
	var err error

	for _, plugin := range p.dbPlugins {
		if err = plugin.store.DeleteBundle(ctx, trustDomainID, mode); err != nil {
			return err
		}
	}

	return err
}

func (p *Plugin) FetchBundle(ctx context.Context, trustDomainID string) (*common.Bundle, error) {
	var (
		bundle *common.Bundle
		err    error
	)

	for _, plugin := range p.dbPlugins {
		if bundle, err = plugin.store.FetchBundle(ctx, trustDomainID); err != nil {
			return nil, err
		}
	}

	return bundle, nil
}

func (p *Plugin) ListBundles(ctx context.Context, req *ListBundlesRequest) (*ListBundlesResponse, error) {
	var (
		resp *ListBundlesResponse
		err  error
	)

	for _, plugin := range p.dbPlugins {
		if resp, err = plugin.store.ListBundles(ctx, req); err != nil {
			return nil, err
		}
	}

	return resp, nil
}
func (p *Plugin) PruneBundle(ctx context.Context, trustDomainID string, expiresBefore time.Time) (changed bool, err error) {
	for _, plugin := range p.dbPlugins {
		if changed, err = plugin.store.PruneBundle(ctx, trustDomainID, expiresBefore); err != nil {
			return false, err
		}
	}

	return changed, nil
}
func (p *Plugin) SetBundle(ctx context.Context, bundle *common.Bundle) (*common.Bundle, error) {
	var (
		retval *common.Bundle
		err    error
	)

	for _, plugin := range p.dbPlugins {
		if retval, err = plugin.store.SetBundle(ctx, bundle); err != nil {
			return nil, err
		}
	}

	return retval, nil
}
func (p *Plugin) UpdateBundle(ctx context.Context, bundle *common.Bundle, mask *common.BundleMask) (*common.Bundle, error) {
	var (
		retval *common.Bundle
		err    error
	)

	for _, plugin := range p.dbPlugins {
		if retval, err = plugin.store.UpdateBundle(ctx, bundle, mask); err != nil {
			return nil, err
		}
	}

	return retval, nil
}

// Keys
func (p *Plugin) TaintX509CA(ctx context.Context, trustDomainID string, subjectKeyIDToTaint string) error {
	var err error

	for _, plugin := range p.dbPlugins {
		if err = plugin.store.TaintX509CA(ctx, trustDomainID, subjectKeyIDToTaint); err != nil {
			return err
		}
	}

	return err
}

func (p *Plugin) RevokeX509CA(ctx context.Context, trustDomainID string, subjectKeyIDToRevoke string) error {
	var err error

	for _, plugin := range p.dbPlugins {
		if err = plugin.store.RevokeX509CA(ctx, trustDomainID, subjectKeyIDToRevoke); err != nil {
			return err
		}
	}

	return err
}

func (p *Plugin) TaintJWTKey(ctx context.Context, trustDomainID string, authorityID string) (*common.PublicKey, error) {
	var (
		pubKey *common.PublicKey
		err    error
	)

	for _, plugin := range p.dbPlugins {
		if pubKey, err = plugin.store.TaintJWTKey(ctx, trustDomainID, authorityID); err != nil {
			return nil, err
		}
	}

	return pubKey, nil
}

func (p *Plugin) RevokeJWTKey(ctx context.Context, trustDomainID string, authorityID string) (*common.PublicKey, error) {
	var (
		pubKey *common.PublicKey
		err    error
	)

	for _, plugin := range p.dbPlugins {
		if pubKey, err = plugin.store.RevokeJWTKey(ctx, trustDomainID, authorityID); err != nil {
			return nil, err
		}
	}

	return pubKey, nil
}

// Entries
func (p *Plugin) CountRegistrationEntries(ctx context.Context, req *CountRegistrationEntriesRequest) (int32, error) {
	var (
		count int32
		err   error
	)

	for _, plugin := range p.dbPlugins {
		if count, err = plugin.store.CountRegistrationEntries(ctx, req); err != nil {
			return 0, err
		}
	}

	return count, nil
}

func (p *Plugin) CreateRegistrationEntry(ctx context.Context, entry *common.RegistrationEntry) (*common.RegistrationEntry, error) {
	var (
		retval *common.RegistrationEntry
		err    error
	)

	for _, plugin := range p.dbPlugins {
		if retval, err = plugin.store.CreateRegistrationEntry(ctx, entry); err != nil {
			return nil, err
		}
	}

	return retval, nil
}

func (p *Plugin) CreateOrReturnRegistrationEntry(ctx context.Context, entry *common.RegistrationEntry) (*common.RegistrationEntry, bool, error) {
	var (
		retval  *common.RegistrationEntry
		success bool
		err     error
	)

	for _, plugin := range p.dbPlugins {
		if retval, success, err = plugin.store.CreateOrReturnRegistrationEntry(ctx, entry); err != nil {
			return retval, success, err
		}
	}

	return retval, success, nil
}

func (p *Plugin) DeleteRegistrationEntry(ctx context.Context, entryID string) (*common.RegistrationEntry, error) {
	var (
		entry *common.RegistrationEntry
		err   error
	)

	for _, plugin := range p.dbPlugins {
		if entry, err = plugin.store.DeleteRegistrationEntry(ctx, entryID); err != nil {
			return nil, err
		}
	}

	return entry, nil
}

func (p *Plugin) FetchRegistrationEntry(ctx context.Context, entryID string) (*common.RegistrationEntry, error) {
	var (
		entry *common.RegistrationEntry
		err   error
	)

	for _, plugin := range p.dbPlugins {
		if entry, err = plugin.store.FetchRegistrationEntry(ctx, entryID); err != nil {
			return nil, err
		}
	}

	return entry, nil
}

func (p *Plugin) FetchRegistrationEntries(ctx context.Context, entryIDs []string) (map[string]*common.RegistrationEntry, error) {
	var (
		entries map[string]*common.RegistrationEntry
		err     error
	)

	for _, plugin := range p.dbPlugins {
		if entries, err = plugin.store.FetchRegistrationEntries(ctx, entryIDs); err != nil {
			return nil, err
		}
	}

	return entries, nil
}

func (p *Plugin) ListRegistrationEntries(ctx context.Context, req *ListRegistrationEntriesRequest) (*ListRegistrationEntriesResponse, error) {
	var (
		resp *ListRegistrationEntriesResponse
		err  error
	)

	for _, plugin := range p.dbPlugins {
		if resp, err = plugin.store.ListRegistrationEntries(ctx, req); err != nil {
			return nil, err
		}
	}

	return resp, nil
}

func (p *Plugin) PruneRegistrationEntries(ctx context.Context, expiresBefore time.Time) error {
	var err error

	for _, plugin := range p.dbPlugins {
		if err = plugin.store.PruneRegistrationEntries(ctx, expiresBefore); err != nil {
			return err
		}
	}

	return err
}

func (p *Plugin) UpdateRegistrationEntry(ctx context.Context, entry *common.RegistrationEntry, mask *common.RegistrationEntryMask) (*common.RegistrationEntry, error) {
	var (
		updatedEntry *common.RegistrationEntry
		err          error
	)

	for _, plugin := range p.dbPlugins {
		if updatedEntry, err = plugin.store.UpdateRegistrationEntry(ctx, entry, mask); err != nil {
			return nil, err
		}
	}

	return updatedEntry, nil
}

// Entries Events
func (p *Plugin) ListRegistrationEntryEvents(ctx context.Context, req *ListRegistrationEntryEventsRequest) (*ListRegistrationEntryEventsResponse, error) {
	var (
		resp *ListRegistrationEntryEventsResponse
		err  error
	)

	for _, plugin := range p.dbPlugins {
		if resp, err = plugin.store.ListRegistrationEntryEvents(ctx, req); err != nil {
			return nil, err
		}
	}

	return resp, nil
}

func (p *Plugin) PruneRegistrationEntryEvents(ctx context.Context, olderThan time.Duration) error {
	var err error

	for _, plugin := range p.dbPlugins {
		if err = plugin.store.PruneRegistrationEntryEvents(ctx, olderThan); err != nil {
			return err
		}
	}

	return err
}

func (p *Plugin) FetchRegistrationEntryEvent(ctx context.Context, eventID uint) (*RegistrationEntryEvent, error) {
	var (
		regEntryEvent *RegistrationEntryEvent
		err           error
	)

	for _, plugin := range p.dbPlugins {
		if regEntryEvent, err = plugin.store.FetchRegistrationEntryEvent(ctx, eventID); err != nil {
			return nil, err
		}
	}

	return regEntryEvent, nil
}

func (p *Plugin) CreateRegistrationEntryEventForTesting(ctx context.Context, event *RegistrationEntryEvent) error {
	var err error

	for _, plugin := range p.dbPlugins {
		if err = plugin.store.CreateRegistrationEntryEventForTesting(ctx, event); err != nil {
			return err
		}
	}

	return err
}

func (p *Plugin) DeleteRegistrationEntryEventForTesting(ctx context.Context, eventID uint) error {
	var err error

	for _, plugin := range p.dbPlugins {
		if err = plugin.store.DeleteRegistrationEntryEventForTesting(ctx, eventID); err != nil {
			return err
		}
	}

	return err
}

// Nodes
func (p *Plugin) CountAttestedNodes(ctx context.Context, req *CountAttestedNodesRequest) (int32, error) {
	var (
		count int32
		err   error
	)

	for _, plugin := range p.dbPlugins {
		if count, err = plugin.store.CountAttestedNodes(ctx, req); err != nil {
			return 0, err
		}
	}

	return count, nil
}

func (p *Plugin) CreateAttestedNode(ctx context.Context, attestedNode *common.AttestedNode) (*common.AttestedNode, error) {
	var (
		retval *common.AttestedNode
		err    error
	)

	for _, plugin := range p.dbPlugins {
		if retval, err = plugin.store.CreateAttestedNode(ctx, attestedNode); err != nil {
			return nil, err
		}
	}

	return retval, nil
}

func (p *Plugin) DeleteAttestedNode(ctx context.Context, spiffeID string) (*common.AttestedNode, error) {
	var (
		attestedNode *common.AttestedNode
		err          error
	)

	for _, plugin := range p.dbPlugins {
		if attestedNode, err = plugin.store.DeleteAttestedNode(ctx, spiffeID); err != nil {
			return nil, err
		}
	}

	return attestedNode, nil
}

func (p *Plugin) FetchAttestedNode(ctx context.Context, spiffeID string) (*common.AttestedNode, error) {
	var (
		attestedNode *common.AttestedNode
		err          error
	)

	for _, plugin := range p.dbPlugins {
		if attestedNode, err = plugin.store.FetchAttestedNode(ctx, spiffeID); err != nil {
			return nil, err
		}
	}

	return attestedNode, nil
}

func (p *Plugin) ListAttestedNodes(ctx context.Context, req *ListAttestedNodesRequest) (*ListAttestedNodesResponse, error) {
	var (
		resp *ListAttestedNodesResponse
		err  error
	)

	for _, plugin := range p.dbPlugins {
		if resp, err = plugin.store.ListAttestedNodes(ctx, req); err != nil {
			return nil, err
		}
	}

	return resp, nil
}

func (p *Plugin) UpdateAttestedNode(ctx context.Context, attestedNode *common.AttestedNode, mask *common.AttestedNodeMask) (*common.AttestedNode, error) {
	var (
		retval *common.AttestedNode
		err    error
	)

	for _, plugin := range p.dbPlugins {
		if retval, err = plugin.store.UpdateAttestedNode(ctx, attestedNode, mask); err != nil {
			return nil, err
		}
	}

	return retval, nil
}

func (p *Plugin) PruneAttestedExpiredNodes(ctx context.Context, expiredBefore time.Time, includeNonReattestable bool) error {
	var err error

	for _, plugin := range p.dbPlugins {
		if err = plugin.store.PruneAttestedExpiredNodes(ctx, expiredBefore, includeNonReattestable); err != nil {
			return err
		}
	}

	return err
}

// Nodes Events
func (p *Plugin) ListAttestedNodeEvents(ctx context.Context, req *ListAttestedNodeEventsRequest) (*ListAttestedNodeEventsResponse, error) {
	var (
		resp *ListAttestedNodeEventsResponse
		err  error
	)

	for _, plugin := range p.dbPlugins {
		if resp, err = plugin.store.ListAttestedNodeEvents(ctx, req); err != nil {
			return nil, err
		}
	}

	return resp, nil
}

func (p *Plugin) PruneAttestedNodeEvents(ctx context.Context, olderThan time.Duration) error {
	var err error

	for _, plugin := range p.dbPlugins {
		if err = plugin.store.PruneAttestedNodeEvents(ctx, olderThan); err != nil {
			return err
		}
	}

	return err
}

func (p *Plugin) FetchAttestedNodeEvent(ctx context.Context, eventID uint) (*AttestedNodeEvent, error) {
	var (
		attestedNodeEvent *AttestedNodeEvent
		err               error
	)

	for _, plugin := range p.dbPlugins {
		if attestedNodeEvent, err = plugin.store.FetchAttestedNodeEvent(ctx, eventID); err != nil {
			return nil, err
		}
	}

	return attestedNodeEvent, nil
}

func (p *Plugin) CreateAttestedNodeEventForTesting(ctx context.Context, event *AttestedNodeEvent) error {
	var err error

	for _, plugin := range p.dbPlugins {
		if err = plugin.store.CreateAttestedNodeEventForTesting(ctx, event); err != nil {
			return err
		}
	}

	return err
}

func (p *Plugin) DeleteAttestedNodeEventForTesting(ctx context.Context, eventID uint) error {
	var err error

	for _, plugin := range p.dbPlugins {
		if err = plugin.store.DeleteAttestedNodeEventForTesting(ctx, eventID); err != nil {
			return err
		}
	}

	return err
}

// Node selectors
func (p *Plugin) GetNodeSelectors(ctx context.Context, spiffeID string, dataConsistency DataConsistency) ([]*common.Selector, error) {
	var (
		selectors []*common.Selector
		err       error
	)

	for _, plugin := range p.dbPlugins {
		if selectors, err = plugin.store.GetNodeSelectors(ctx, spiffeID, dataConsistency); err != nil {
			return nil, err
		}
	}

	return selectors, nil
}

func (p *Plugin) ListNodeSelectors(ctx context.Context, req *ListNodeSelectorsRequest) (*ListNodeSelectorsResponse, error) {
	var (
		resp *ListNodeSelectorsResponse
		err  error
	)

	for _, plugin := range p.dbPlugins {
		if resp, err = plugin.store.ListNodeSelectors(ctx, req); err != nil {
			return nil, err
		}
	}

	return resp, nil
}

func (p *Plugin) SetNodeSelectors(ctx context.Context, spiffeID string, selectors []*common.Selector) error {
	var err error

	for _, plugin := range p.dbPlugins {
		if err = plugin.store.SetNodeSelectors(ctx, spiffeID, selectors); err != nil {
			return err
		}
	}

	return err
}

// Tokens
func (p *Plugin) CreateJoinToken(ctx context.Context, token *JoinToken) error {
	var err error

	for _, plugin := range p.dbPlugins {
		if err = plugin.store.CreateJoinToken(ctx, token); err != nil {
			return err
		}
	}

	return err
}

func (p *Plugin) DeleteJoinToken(ctx context.Context, token string) error {
	var err error

	for _, plugin := range p.dbPlugins {
		if err = plugin.store.DeleteJoinToken(ctx, token); err != nil {
			return err
		}
	}

	return err
}
func (p *Plugin) FetchJoinToken(ctx context.Context, token string) (*JoinToken, error) {
	var (
		retval *JoinToken
		err    error
	)

	for _, plugin := range p.dbPlugins {
		if retval, err = plugin.store.FetchJoinToken(ctx, token); err != nil {
			return nil, err
		}
	}

	return retval, err
}

func (p *Plugin) PruneJoinTokens(ctx context.Context, expiresBefore time.Time) error {
	var err error

	for _, plugin := range p.dbPlugins {
		if err = plugin.store.PruneJoinTokens(ctx, expiresBefore); err != nil {
			return err
		}
	}

	return err
}

// Federation Relationships
func (p *Plugin) CreateFederationRelationship(ctx context.Context, fedRelationship *FederationRelationship) (*FederationRelationship, error) {
	var (
		retval *FederationRelationship
		err    error
	)

	for _, plugin := range p.dbPlugins {
		if retval, err = plugin.store.CreateFederationRelationship(ctx, fedRelationship); err != nil {
			return nil, err
		}
	}

	return retval, nil
}

func (p *Plugin) FetchFederationRelationship(ctx context.Context, td spiffeid.TrustDomain) (*FederationRelationship, error) {
	var (
		fedRelationship *FederationRelationship
		err             error
	)

	for _, plugin := range p.dbPlugins {
		if fedRelationship, err = plugin.store.FetchFederationRelationship(ctx, td); err != nil {
			return nil, err
		}
	}

	return fedRelationship, nil
}

func (p *Plugin) ListFederationRelationships(ctx context.Context, req *ListFederationRelationshipsRequest) (*ListFederationRelationshipsResponse, error) {
	var (
		resp *ListFederationRelationshipsResponse
		err  error
	)

	for _, plugin := range p.dbPlugins {
		if resp, err = plugin.store.ListFederationRelationships(ctx, req); err != nil {
			return nil, err
		}
	}

	return resp, nil
}

func (p *Plugin) DeleteFederationRelationship(ctx context.Context, td spiffeid.TrustDomain) error {
	var err error

	for _, plugin := range p.dbPlugins {
		if err = plugin.store.DeleteFederationRelationship(ctx, td); err != nil {
			return err
		}
	}

	return err
}

func (p *Plugin) UpdateFederationRelationship(ctx context.Context, fedRelationship *FederationRelationship, mask *types.FederationRelationshipMask) (*FederationRelationship, error) {
	var (
		retval *FederationRelationship
		err    error
	)

	for _, plugin := range p.dbPlugins {
		if retval, err = plugin.store.UpdateFederationRelationship(ctx, fedRelationship, mask); err != nil {
			return nil, err
		}
	}

	return retval, nil
}

// CA Journals
func (p *Plugin) SetCAJournal(ctx context.Context, caJournal *CAJournal) (*CAJournal, error) {
	var (
		retval *CAJournal
		err    error
	)

	for _, plugin := range p.dbPlugins {
		if retval, err = plugin.store.SetCAJournal(ctx, caJournal); err != nil {
			return nil, err
		}
	}

	return retval, nil
}

func (p *Plugin) FetchCAJournal(ctx context.Context, activeX509AuthorityID string) (*CAJournal, error) {
	var (
		retval *CAJournal
		err    error
	)

	for _, plugin := range p.dbPlugins {
		if retval, err = plugin.store.FetchCAJournal(ctx, activeX509AuthorityID); err != nil {
			return nil, err
		}
	}

	return retval, nil
}

func (p *Plugin) PruneCAJournals(ctx context.Context, allCAsExpireBefore int64) error {
	var err error
	for _, plugin := range p.dbPlugins {
		if err = plugin.store.PruneCAJournals(ctx, allCAsExpireBefore); err != nil {
			return err
		}
	}

	return err
}

func (p *Plugin) ListCAJournalsForTesting(ctx context.Context) ([]*CAJournal, error) {
	var (
		journals []*CAJournal
		err      error
	)

	for _, plugin := range p.dbPlugins {
		if journals, err = plugin.store.ListCAJournalsForTesting(ctx); err != nil {
			return nil, err
		}
	}

	return journals, nil
}
