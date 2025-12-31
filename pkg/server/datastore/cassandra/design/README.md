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
- Isolatable debuggable tests
- Bundles

## To Be Implemented
- Pagination
- sqlite tests in pluggable integration test mode
- Bundle federation relationship and deletion interactions
- general approach to consistency
- use gocql, not scylladb 
- replace migrations
- configurable topology strategy
- allow running test suite after failure
- teardown

## Test suite progress

#### Area: Bundles
- [x] TestBundleCRUD()
- [x] TestCountBundles()
- [x] TestBundlePrune()
- [x] TestSetBundle()
- [ ] TestListBundlesWithPagination()
- [ ] TestDeleteBundleRestrictedByRegistrationEntries()
- [ ] TestDeleteBundleDeleteRegistrationEntries()
- [ ] TestDeleteBundleDissociateRegistrationEntries()

#### Area: RegistrationEntry
- [x] TestCountRegistrationEntries()
- [x] TestCreateRegistrationEntry()
- [ ] TestCreateOrReturnRegistrationEntry()
- [x] TestCreateInvalidRegistrationEntry()
- [x] TestFetchRegistrationEntry()
- [x] TestFetchRegistrationEntryDoesNotExist()
- [x] TestFetchRegistrationEntries()
- [ ] TestPruneRegistrationEntries()
- [x] TestFetchInexistentRegistrationEntry()
- [ ] TestListRegistrationEntries()
  - [x] with_partial_page_with_pagination
  - [x] with_partial_page_without_pagination
  - [x] with_full_page_with_pagination_read-only
  - [x] with_partial_page_with_pagination_read-only
  - [x] with_full_page_with_pagination
  - [x] with_full_page_without_pagination
  - [x] with_full_page_with_pagination_read-only
  - [x] with_full_page_without_pagination_read-only
  - [x] with_page_and_a_half_with_pagination
  - [x] with_page_and_a_half_without_pagination
  - [x] with_page_and_a_half_with_pagination_read-only
  - [x] with_page_and_a_half_without_pagination_read-only
  - [ ] by_parent_ID_with_pagination
  - [x] by_parent_ID_without_pagination
  - [ ] by_parent_ID_with_pagination_read-only
  - [x] by_parent_ID_without_pagination_read-only
- [ ] TestUpdateRegistrationEntry()
- [ ] TestUpdateRegistrationEntryWithStoreSvid()
- [ ] TestUpdateRegistrationEntryWithMask()
- [ ] TestDeleteRegistrationEntry()
- [ ] TestListParentIDEntries()
- [ ] TestListSelectorEntries()
- [ ] TestListEntriesBySelectorSubset()
- [ ] TestListSelectorEntriesSuperset()
- [ ] TestListEntriesBySelectorMatchAny()
- [ ] TestListEntriesByFederatesWithExact()
- [ ] TestListEntriesByFederatesWithSubset()
- [ ] TestListEntriesByFederatesWithMatchAny()
- [ ] TestListEntriesByFederatesWithSuperset()
- [ ] TestRegistrationEntriesFederatesWithAgainstMissingBundle()
- [ ] TestRegistrationEntriesFederatesWithSuccess()
- [ ] TestListRegistrationEntryEvents()
- [ ] TestPruneRegistrationEntryEvents()

#### Area: CA Journal
- [ ] TestSetCAJournal()
- [ ] TestFetchCAJournal()
- [ ] TestPruneCAJournal()

#### Area: Federation Relationships
- [ ] TestDeleteFederationRelationship()
- [ ] TestFetchFederationRelationship()
- [ ] TestCreateFederationRelationship()
- [ ] TestListFederationRelationships()
- [ ] TestUpdateFederationRelationship()

#### Area: X509CA
- [ ] TestTaintX509CA()
- [ ] TestRevokeX509CA()

#### Area: JWTKey
- [ ] TestTaintJWTKey()
- [ ] TestRevokeJWTKey()

#### Area: Join Token
- [ ] TestCreateJoinToken()
- [ ] TestCreateAndFetchJoinToken()
- [ ] TestDeleteJoinToken()
- [ ] TestPruneJoinTokens()

#### Area: Nodes
- [ ] TestCreateAttestedNode()
- [ ] TestFetchAttestedNodeMissing()
- [ ] TestListAttestedNodes()
- [ ] TestUpdateAttestedNode()
- [ ] TestPruneAttestedExpiredNodes()
- [ ] TestDeleteAttestedNode()
- [ ] TestListAttestedNodeEvents()
- [ ] TestPruneAttestedNodeEvents()
- [ ] TestNodeSelectors()
- [ ] TestListNodeSelectors()
- [ ] TestSetNodeSelectorsUnderLoad()
- [ ] TestCountAttestedNodes()

#### Area: Configuration/General
- [ ] TestInvalidPluginConfiguration()
- [ ] TestInvalidAWSConfiguration()
- [ ] TestInvalidMySQLConfiguration()
- [ ] TestRace()

