package cassandra

import (
	"context"
	"crypto/x509"

	"github.com/spiffe/spire/pkg/common/x509util"
	"github.com/spiffe/spire/proto/spire/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Effectively a 1:1 copy from pkg/server/datastore/sqlstore/sqlstore.go:1451
func (p *plugin) TaintX509CA(ctx context.Context, trustDomainID string, subjectKeyIDToTaint string) error {
	bundle, err := p.FetchBundle(ctx, trustDomainID)
	if err != nil {
		return err
	}

	found := false
	for _, eachRootCA := range bundle.RootCas {
		x509CA, err := x509.ParseCertificate(eachRootCA.DerBytes)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to parse rootCA: %v", err)
		}

		caSubjectKeyID := x509util.SubjectKeyIDToString(x509CA.SubjectKeyId)
		if subjectKeyIDToTaint != caSubjectKeyID {
			continue
		}

		if eachRootCA.TaintedKey {
			return status.Errorf(codes.InvalidArgument, "root CA is already tainted")
		}

		found = true
		eachRootCA.TaintedKey = true
	}

	if !found {
		return status.Error(codes.NotFound, "no ca found with provided subject key ID")
	}

	bundle.SequenceNumber++

	_, err = updateBundle(p.db.session, bundle, nil)
	return err
}

// Effectively a 1:1 copy from pkg/server/datastore/sqlstore/sqlstore.go:1488
func (p *plugin) RevokeX509CA(ctx context.Context, trustDomainID string, subjectKeyIDToRevoke string) error {
	bundle, err := p.FetchBundle(ctx, trustDomainID)
	if err != nil {
		return err
	}

	keyFound := false
	var rootCAs []*common.Certificate
	for _, ca := range bundle.RootCas {
		cert, err := x509.ParseCertificate(ca.DerBytes)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to parse root CA: %v", err)
		}

		caSubjectKeyID := x509util.SubjectKeyIDToString(cert.SubjectKeyId)
		if subjectKeyIDToRevoke == caSubjectKeyID {
			if !ca.TaintedKey {
				return status.Error(codes.InvalidArgument, "it is not possible to revoke an untainted root CA")
			}
			keyFound = true
			continue
		}

		rootCAs = append(rootCAs, ca)
	}

	if !keyFound {
		return status.Error(codes.NotFound, "no root CA found with provided subject key ID")
	}

	bundle.RootCas = rootCAs
	bundle.SequenceNumber++

	if _, err := updateBundle(p.db.session, bundle, nil); err != nil {
		return status.Errorf(codes.Internal, "failed to update bundle: %v", err)
	}

	return nil
}

// Effectively a 1:1 copy from pkg/server/datastore/sqlstore/sqlstore.go:1527
func (p *plugin) TaintJWTKey(ctx context.Context, trustDomainID string, authorityID string) (*common.PublicKey, error) {
	bundle, err := p.FetchBundle(ctx, trustDomainID)
	if err != nil {
		return nil, err
	}

	var taintedKey *common.PublicKey
	for _, jwtKey := range bundle.JwtSigningKeys {
		if jwtKey.Kid != authorityID {
			continue
		}

		if jwtKey.TaintedKey {
			return nil, status.Error(codes.InvalidArgument, "key is already tainted")
		}

		// Check if a JWT Key with the provided keyID was already
		// tainted in this loop. This is purely defensive since we do not
		// allow to have repeated key IDs.
		if taintedKey != nil {
			return nil, status.Error(codes.Internal, "another JWT Key found with the same KeyID")
		}
		taintedKey = jwtKey
		jwtKey.TaintedKey = true
	}

	if taintedKey == nil {
		return nil, status.Error(codes.NotFound, "no JWT Key found with provided key ID")
	}

	bundle.SequenceNumber++
	if _, err := updateBundle(p.db.session, bundle, nil); err != nil {
		return nil, err
	}

	return taintedKey, nil
}

// Effectively a 1:1 copy from pkg/server/datastore/sqlstore/sqlstore.go:1565
func (p *plugin) RevokeJWTKey(ctx context.Context, trustDomainID string, authorityID string) (*common.PublicKey, error) {
	bundle, err := p.FetchBundle(ctx, trustDomainID)
	if err != nil {
		return nil, err
	}

	var publicKeys []*common.PublicKey
	var revokedKey *common.PublicKey
	for _, key := range bundle.JwtSigningKeys {
		if key.Kid == authorityID {
			// Check if a JWT Key with the provided keyID was already
			// found in this loop. This is purely defensive since we do not
			// allow to have repeated key IDs.
			if revokedKey != nil {
				return nil, status.Error(codes.Internal, "another key found with the same KeyID")
			}

			if !key.TaintedKey {
				return nil, status.Error(codes.InvalidArgument, "it is not possible to revoke an untainted key")
			}

			revokedKey = key
			continue
		}
		publicKeys = append(publicKeys, key)
	}
	bundle.JwtSigningKeys = publicKeys

	if revokedKey == nil {
		return nil, status.Error(codes.NotFound, "no JWT Key found with provided key ID")
	}

	bundle.SequenceNumber++
	if _, err := updateBundle(p.db.session, bundle, nil); err != nil {
		return nil, err
	}

	return revokedKey, nil
}
