package cassandra

import (
	"context"
	"errors"
	"time"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
	"github.com/spiffe/spire/pkg/server/datastore"
)

func (p *plugin) CreateJoinToken(ctx context.Context, token *datastore.JoinToken) error {
	if token == nil || token.Token == "" || token.Expiry.IsZero() {
		return errors.New("token and expiry are required")
	}

	createQ := `INSERT INTO join_tokens (join_token, expiry) VALUES (?, ?) IF NOT EXISTS`
	applied, err := p.db.session.Query(createQ,
		token.Token,
		token.Expiry.Unix(),
	).ScanCAS()
	if err != nil {
		return err
	}

	if !applied {
		return newWrappedCassandraError(errors.New("join token already exists"))
	}

	return nil
}

func (p *plugin) DeleteJoinToken(ctx context.Context, token string) error {
	deleteQ := `DELETE FROM join_tokens WHERE join_token = ? IF EXISTS`
	applied, err := p.db.session.Query(deleteQ, token).ScanCAS()
	if err != nil {
		return err
	}

	if !applied {
		return newWrappedCassandraError(errors.New("join token not found"))
	}

	return nil
}

func (p *plugin) FetchJoinToken(ctx context.Context, token string) (*datastore.JoinToken, error) {
	var (
		jt  datastore.JoinToken
		exp int64
	)

	findQ := `SELECT join_token, expiry FROM join_tokens WHERE join_token = ?`

	if err := p.db.session.Query(findQ, token).Scan(
		&jt.Token,
		&exp,
	); err != nil {
		if errors.Is(err, gocql.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	jt.Expiry = time.Unix(exp, 0)

	return &jt, nil
}

func (p *plugin) PruneJoinTokens(ctx context.Context, t time.Time) error {
	findExpired := `SELECT join_token FROM join_tokens WHERE expiry < ? ALLOW FILTERING`
	scanner := p.db.session.Query(findExpired, t.Unix()).Iter().Scanner()
	tokens := make([]string, 0)
	for scanner.Next() {
		var token string
		if err := scanner.Scan(&token); err != nil {
			return err
		}
		tokens = append(tokens, token)
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	q := `DELETE FROM join_tokens WHERE join_token IN ?`
	if err := p.db.session.Query(q, tokens).Exec(); err != nil {
		return err
	}

	return nil
}
