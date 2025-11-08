package cassandra

import (
	"context"
	"time"

	"github.com/gogo/status"
	"github.com/spiffe/spire/pkg/server/datastore"
	"github.com/spiffe/spire/proto/spire/common"
	"google.golang.org/grpc/codes"
)

func (db *plugin) CountRegistrationEntries(context.Context, *datastore.CountRegistrationEntriesRequest) (int32, error) {
	return 0, NotImplementedErr
}

func (db *plugin) CreateRegistrationEntry(context.Context, *common.RegistrationEntry) (*common.RegistrationEntry, error) {
	return nil, NotImplementedErr
}

func (db *plugin) CreateOrReturnRegistrationEntry(context.Context, *common.RegistrationEntry) (*common.RegistrationEntry, bool, error) {
	return nil, false, NotImplementedErr
}

func (db *plugin) DeleteRegistrationEntry(ctx context.Context, entryID string) (*common.RegistrationEntry, error) {
	return nil, NotImplementedErr
}

func (db *plugin) FetchRegistrationEntry(ctx context.Context, entryID string) (*common.RegistrationEntry, error) {
	return nil, NotImplementedErr
}

func (db *plugin) FetchRegistrationEntries(ctx context.Context, entryIDs []string) (map[string]*common.RegistrationEntry, error) {
	return nil, NotImplementedErr
}

func (db *plugin) ListRegistrationEntries(ctx context.Context, req *datastore.ListRegistrationEntriesRequest) (*datastore.ListRegistrationEntriesResponse, error) {
	if req.Pagination != nil && req.Pagination.PageSize == 0 {
		return nil, status.Error(codes.InvalidArgument, "cannot paginate with pagesize = 0")
	}
	if req.BySelectors != nil && len(req.BySelectors.Selectors) == 0 {
		return nil, status.Error(codes.InvalidArgument, "cannot list by empty selector set")
	}

	return nil, NotImplementedErr
}

func (db *plugin) PruneRegistrationEntries(ctx context.Context, expiresBefore time.Time) error {
	return NotImplementedErr
}

func (db *plugin) UpdateRegistrationEntry(context.Context, *common.RegistrationEntry, *common.RegistrationEntryMask) (*common.RegistrationEntry, error) {
	return nil, NotImplementedErr
}
