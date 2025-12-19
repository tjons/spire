# Cassandra datastore implementation

This is a proof-of-concept implementation of Apache Cassandra as a backing datastore for SPIRE server. 

## Cassandra Schema



## Implemented
- Alternative datastore configuration loading via experimental settings.
- Interface type for the cassandra implementation and restructuring of the existing plugin where necessary
- Basic scaffolding of methods without implementation
- Decoupling of test harness from sqlstore package to allow multiple DataStore implementations to pass a common battery of tests
- `*-pluggable` versions of existing datastore tests for MySQL and Postgres against the DataStore interface integration tests
- Validation of replication for postgres in pluggable mode
- Validation of MySQL without replication in pluggable mode
- Validation of MySQL with replication in pluggable mode
- Cassandra tests with pluggable mode

## To Be Implemented
- Pagination
- sqlite tests in pluggable integration test mode