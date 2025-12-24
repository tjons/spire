package cassandra

import (
	"context"

	"github.com/spiffe/spire/proto/spire/common"
)

func (p *plugin) TaintX509CA(ctx context.Context, trustDomainID string, subjectKeyIDToTaint string) error {
	return NotImplementedErr
}

func (p *plugin) RevokeX509CA(ctx context.Context, trustDomainID string, subjectKeyIDToRevoke string) error {
	return NotImplementedErr
}

func (p *plugin) TaintJWTKey(ctx context.Context, trustDomainID string, authorityID string) (*common.PublicKey, error) {
	return nil, NotImplementedErr
}

func (p *plugin) RevokeJWTKey(ctx context.Context, trustDomainID string, authorityID string) (*common.PublicKey, error) {
	return nil, NotImplementedErr

}
