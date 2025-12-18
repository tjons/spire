# Redesigning the SPIRE server schema around denormalization

## Denormalization

Denormalization is a core tenant of the Cassandra data model. Normalization is one of the key features that makes relational databases work the way we expect them to; Cassandra is a different beast. One downside of data normalization and referential schemas is the requirement to `join` various tables together at read time to retrieve a required set of rows.

## The Relational SPIRE server schema

The existing, supported SPIRE server schema is _heavily_ relational, with relationships implemented via ID columns stored in the datastore itself. However, foreign keys are surprisingly lacking, meaning that referencial integrity is achieved in the application layer. This will present a challenge for the Cassandra implementation, as the cassandra implementation will need to provide similarly robust guarentees in the driver code itself. This is not trivial and will require a high level of attention to detail.

However, the Cassandra database _can_ provide a **significant** improvement in performance by leveraging denormalization to remove the need to join tables together.

## Relationships in the RDBMS schema

### ORM-defined relationships

- `bundles` have a `many2many` relationship with `federated_registration_entries` (pkg/server/datastore/sqlstore/models.go:22) 
- `registered_entries` have a `many2many` relationship with `federated_registration_entries` ((pkg/server/datastore/sqlstore/models.go:81)
- `registered_entries` have a one to many(?) relationship to Bundles (pkg/server/datastore/sqlstore/models.go:81)
- `registered_entries` have a one to many relationship to `Selectors` (pkg/server/datastore/sqlstore/models.go:80)
- `registered_entries` have a one to many relationship to `DNSNames` (pkg/server/datastore/sqlstore/models.go:87)
- `AttestedNodes` have a one to many relationship to `NodeSelectors` (pkg/server/datastore/sqlstore/models.go:37)
- `AttestedNodeEvents` have a one-to-one relationship to `AttestedNodes` (pkg/server/datastore/models.go:49)
- `NodeSelector` has a one-to-one relationship to `AttestedNodes` (pkg/server/datastore/models.go:61)
- `RegisteredEntryEvents` have a one-to-one relationship to `RegisteredEntries` (pkg/server/datastore/models.go:108)
- `Selectors` have a one-to-one relationship to `RegisteredEntries` (pkg/server/datastore/models.go:127)
- `DNSNames` have a one-to-one relationship to `RegisteredEntries` (pkg/server/datastore/models.go:136)

### Foreign key usage

None of the above relationships are implemented via foreign keys:
```
spire=# SELECT
    tc.table_schema,
    tc.constraint_name,
    tc.table_name,
    kcu.column_name,
    ccu.table_schema AS foreign_table_schema,
    ccu.table_name AS foreign_table_name,
    ccu.column_name AS foreign_column_name
FROM information_schema.table_constraints AS tc
JOIN information_schema.key_column_usage AS kcu
    ON tc.constraint_name = kcu.constraint_name
    AND tc.table_schema = kcu.table_schema
JOIN information_schema.constraint_column_usage AS ccu
    ON ccu.constraint_name = tc.constraint_name
WHERE tc.constraint_type = 'FOREIGN KEY'
    AND tc.table_schema='spire';
 table_schema | constraint_name | table_name | column_name | foreign_table_schema | foreign_table_name | foreign_column_name
--------------+-----------------+------------+-------------+----------------------+--------------------+---------------------
(0 rows)

spire=#
```
