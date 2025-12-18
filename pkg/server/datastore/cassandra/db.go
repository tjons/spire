package cassandra

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	agocql "github.com/apache/cassandra-gocql-driver/v2"
	"github.com/gocql/gocql"
	"github.com/hashicorp/hcl"
	"github.com/scylladb/gocqlx/v3"
	"github.com/scylladb/gocqlx/v3/migrate"
	"github.com/sirupsen/logrus"
	"github.com/spiffe/spire/pkg/server/datastore"

	"github.com/spiffe/spire/pkg/server/datastore/cassandra/migrations"
)

const (
	PluginName = "cassandra"
)

var NotImplementedErr = errors.New("cassandra datastore does not support this method")

type configuration struct {
	CassandraAddresses []string `hcl:"cassandra_addresses"`
	CassandraKeyspace  string   `hcl:"cassandra_keyspace"`
	Mode               string   `hcl:"mode"`
}

type cassandraDB struct {
	cfg     *configuration
	rwLock  *sync.Mutex
	log     logrus.FieldLogger
	session *gocql.Session
}

type plugin struct {
	rlock  *sync.Mutex
	rwLock *sync.Mutex
	log    logrus.FieldLogger
	cfg    *configuration
	db     *cassandraDB
}

func New(log logrus.FieldLogger) datastore.DataStore {
	return &plugin{
		rlock:  &sync.Mutex{},
		rwLock: &sync.Mutex{},
		log:    log,
	}
}

func (p *plugin) Close() error {
	return nil
}

func (p *plugin) Configure(ctx context.Context, hclConfiguration string) error {
	cfg := &configuration{}
	if err := hcl.Decode(cfg, hclConfiguration); err != nil {
		return err
	}

	p.cfg = cfg

	return p.openConnections(p.cfg)
}

func (p *plugin) openConnections(config *configuration) error {
	if p.cfg == nil {
		return errors.New("configuration not set")
	}

	return p.openConnection(config)
}

func (p *plugin) openConnection(config *configuration) (err error) {
	p.rwLock.Lock()
	defer p.rwLock.Unlock()

	if err := p.applyMigrations(); err != nil {
		return err
	}

	db := &cassandraDB{
		cfg: config,
		log: p.log,
	}

	clusterConfig := gocql.NewCluster(config.CassandraAddresses...)
	clusterConfig.Keyspace = config.CassandraKeyspace
	clusterConfig.Consistency = gocql.Quorum
	db.session, err = clusterConfig.CreateSession()
	if err != nil {
		return err
	}

	p.db = db

	return nil
}

func (p *plugin) applyMigrations() error {
	bootstrapCluster := agocql.NewCluster(p.cfg.CassandraAddresses...)
	bootstrapSession, err := bootstrapCluster.CreateSession()
	if err != nil {
		return err
	}

	createKeyspaceQuery := fmt.Sprintf("CREATE KEYSPACE IF NOT EXISTS %s WITH REPLICATION = {'class': 'SimpleStrategy', 'replication_factor': 1}", p.cfg.CassandraKeyspace) // TODO(tjons): this will be obviously wrong for a production environment

	if err = bootstrapSession.Query(createKeyspaceQuery).Exec(); err != nil {
		return fmt.Errorf("failed to create keyspace: %w", err)
	}
	bootstrapSession.Close()

	cluster := gocql.NewCluster(p.cfg.CassandraAddresses...)
	cluster.Keyspace = p.cfg.CassandraKeyspace

	session, err := gocqlx.WrapSession(cluster.CreateSession())
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	log := func(mctx context.Context, session gocqlx.Session, ev migrate.CallbackEvent, name string) error {
		switch ev {
		case migrate.BeforeMigration:
			p.log.Infof("Starting migration: %s", name)
		case migrate.AfterMigration:
			p.log.Infof("Completed migration: %s", name)
		}

		return nil
	}
	reg := migrate.CallbackRegister{}
	migrate.Callback = reg.Callback

	pending, err := migrate.Pending(context.Background(), session, migrations.Migrations)
	if err != nil {
		return fmt.Errorf("failed to check pending migrations: %w", err)
	}

	if len(pending) == 0 {
		p.log.Info("No pending migrations found.")
		return nil
	}

	premigrationMessage := "Pending migrations:\n---\n"
	for _, m := range pending {
		reg.Add(migrate.BeforeMigration, m.Name, log)
		reg.Add(migrate.AfterMigration, m.Name, log)

		premigrationMessage += fmt.Sprintf("\n- %s (checksum: %s)", m.Name, m.Checksum)
	}
	premigrationMessage += "\n---\n"
	p.log.Infof(premigrationMessage)

	if err := migrate.FromFS(context.Background(), session, migrations.Migrations); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}

type Model struct {
	ID        uint
	CreatedAt time.Time
	UpdatedAt time.Time
}
