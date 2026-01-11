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
  - need to audit all calls for appropriate consistency
- use gocql, not scylladb 
- replace migrations
- configurable topology strategy
- allow running test suite after failure
- teardown
- handling the new Validate() process without introducing package dependency issues

## Some challenges
There are three distinct issues that are common across multiple resource types so far that do not have easy answers:
1. Pagination
2. Filtering
3. Ordering in multi-row queries

This is a little self dialog about them.

Pagination is hard because cassandra's internal paging mechanisms don't follow
the ordering semantics that sql DB's do, and denormalization makes paging hard if
we denormalize to multi-row partitions, via clustering keys. It could be more correct,
although certainly harder to implement, if we used a page size of 1, read records one at
a time, and then we were able to implement user "page size" on top of this silly 
single item pages querying thing. Pagination tokens are also hard because the SQL tests
expect a known value for the "next page", and all have hard coded strings of ints. the
solution here is going to be refactoring those tests to check insead for a paging token
that is not an empty string instead.

Filtering: this one is a total bear. For almost all the resources, storage-attached indexes 
do everything we need, but they prevent us from denormalizing because the index only gets
attached to a single row. There are some funky workarounds, where we could have multiple
rows per partition, one for each search term + match combo we need to support. it would
be weird and ugly. For somethign like trust domain filters on registration entries, 
it would look like this: each trust domain that the entry federates with would be a new
row, with a column called "search_by_ftd" that would be the _value_ of the federated trust
domain. each "indexed" row would still have the full set of other values in their standard
form. we'd also have to index this for each matcher type. would probably require us to use
"pseudoversioning" of the data as well, to avoid races in "read before write". might be
able to do some fancy tricks where we prebuild every single search term based on the matchers
that can be supported, and then just use all of that. this would result in row explosion, but
it would solve some of these issues. for example, an entry that federates with:
- spiffe://td1
- spiffe://td2
would have these `filter_val` rows:
- `ftd_match_exact_spiffe://td1_spiffe://td2`
- `ftd_match_any_spiffe://td1`
- `ftd_match_any_spiffe://td2`
- `ftd_match_any_spiffe://td1_spiffe://td2`
- `ftd_match_superset_spiffe://td1_spiffe://td2`
- `ftd_match_subset_spiffe://td1_spiffe://td2`

Where this gets hard... when multiple selectors are ANDED together like this, it increases
the number of rows exponentially. so, something like an entry that has selectors:
- type:a value:b
- type:b value:c
would have these `filter_val` rows:
- `stv_match_exact_type_a_value_b__type_b_value_c`
- `stv_match_any_type_a_value_b`
- `stv_match_any_type_b_value_c`
- `stv_match_superset_type_a_value_b`
- `stv_match_subset_type_a_value_b__type_b_value_c`

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
- [x] TestCreateOrReturnRegistrationEntry()
- [x] TestCreateInvalidRegistrationEntry()
- [x] TestFetchRegistrationEntry()
- [x] TestFetchRegistrationEntryDoesNotExist()
- [x] TestFetchRegistrationEntries()
- [x] TestPruneRegistrationEntries()
- [x] TestFetchInexistentRegistrationEntry()
- [x] TestListRegistrationEntries()
- [ ] TestUpdateRegistrationEntry()
- [ ] TestUpdateRegistrationEntryWithStoreSvid()
- [ ] TestUpdateRegistrationEntryWithMask()
- [x] TestDeleteRegistrationEntry()
- [x] TestListParentIDEntries()
- [x] TestListSelectorEntries()
- [x] TestListEntriesBySelectorSubset()
- [x] TestListSelectorEntriesSuperset()
- [x] TestListEntriesBySelectorMatchAny()
- [x] TestListEntriesByFederatesWithExact()
- [x] TestListEntriesByFederatesWithSubset()
- [x] TestListEntriesByFederatesWithMatchAny()
- [x] TestListEntriesByFederatesWithSuperset()
- [x] TestRegistrationEntriesFederatesWithAgainstMissingBundle()
- [x] TestRegistrationEntriesFederatesWithSuccess()
- [ ] TestListRegistrationEntryEvents()
- [ ] TestPruneRegistrationEntryEvents()

#### Area: CA Journal
- [x] TestSetCAJournal()
- [x] TestFetchCAJournal()
- [x] TestPruneCAJournal()

#### Area: Federation Relationships
- [x] TestDeleteFederationRelationship()
- [x] TestFetchFederationRelationship()
- [x] TestCreateFederationRelationship()
- [ ] TestListFederationRelationships()
- [x] TestUpdateFederationRelationship()

#### Area: X509CA
- [x] TestTaintX509CA()
- [x] TestRevokeX509CA()

#### Area: JWTKey
- [x] TestTaintJWTKey()
- [x] TestRevokeJWTKey()

#### Area: Join Token
- [x] TestCreateJoinToken()
- [x] TestCreateAndFetchJoinToken()
- [x] TestDeleteJoinToken()
- [x] TestPruneJoinTokens()

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

