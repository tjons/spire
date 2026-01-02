CREATE TABLE IF NOT EXISTS registered_entries (
    created_at timestamp,
    updated_at timestamp,
    entry_id text,
    spiffe_id text,
    parent_id text,
    ttl int,
    admin boolean,
    downstream boolean,
    expiry bigint,
    revision_number bigint,
    store_svid boolean,
    hint text,
    jwt_svid_ttl int,
    selector_types list<text>,
    selector_values list<text>,
    dns_names set<text>,
    federated_trust_domains set<text>,
    federated_trust_domains_m map<int, text>,
    PRIMARY KEY (entry_id)
);
--- TODO(tjons) whats the default sort order on disk of the above table? I fear we need
--- to sort these by creation time... ugh

CREATE INDEX IF NOT EXISTS parent_id_idx ON registered_entries (parent_id) USING 'sai';
CREATE INDEX IF NOT EXISTS spiffe_id_idx ON registered_entries (spiffe_id) USING 'sai';
CREATE INDEX IF NOT EXISTS hint_idx ON registered_entries (hint) USING 'sai';
CREATE INDEX IF NOT EXISTS federated_trust_domain_idx ON registered_entries (federated_trust_domains) USING 'sai';
CREATE INDEX IF NOT EXISTS downstream_idx ON registered_entries (downstream) USING 'sai';
CREATE INDEX IF NOT EXISTS selector_types_idx ON registered_entries (selector_types) USING 'sai';
CREATE INDEX IF NOT EXISTS selector_values_idx ON registered_entries (selector_values) USING 'sai';
CREATE INDEX IF NOT EXISTS expiry_idx ON registered_entries (expiry) USING 'sai';


--- TODO:(tjons) should this be collapsed with the above?
CREATE TABLE IF NOT EXISTS registered_entries_events (
    id int,
    created_at timestamp,
    updated_at timestamp,
    entry_id varchar,
    PRIMARY KEY((id), entry_id)
);

--- TODO(tjons): this should _almost certainly_ be collapsed with the above
CREATE TABLE IF NOT EXISTS selectors (
    id int,
    created_at timestamp,
    updated_at timestamp,
    registered_entry_id int,
    type varchar,
    value varchar,
    PRIMARY KEY((registered_entry_id, type, value))
);

CREATE TABLE IF NOT EXISTS attested_node_entries (
    id int,
    created_at timestamp,
    updated_at timestamp,
    spiffe_id varchar,
    data_type varchar,
    serial_number varchar,
    expires_at timestamp,
    new_serial_number varchar,
    new_expires_at timestamp,
    can_reattest boolean,
    PRIMARY KEY (spiffe_id)
);

CREATE TABLE IF NOT EXISTS attested_node_entries_events (
    id int,
    created_at timestamp,
    updated_at timestamp,
    spiffe_id varchar,
    PRIMARY KEY (spiffe_id, id)
);

CREATE TABLE IF NOT EXISTS bundles (
    created_at timestamp STATIC,
    updated_at timestamp STATIC,
    trust_domain varchar,
    data blob STATIC, --- this is static because the bundle data is the same across all rows
    federated_entry_id varchar,
    PRIMARY KEY (trust_domain, federated_entry_id),
);
CREATE INDEX IF NOT EXISTS federated_entry_id_idx ON bundles (federated_entry_id) USING 'sai';

CREATE TABLE IF NOT EXISTS ca_journals (
    id int,
    created_at timestamp,
    updated_at timestamp,
    data blob,
    active_x509_authority_id varchar,
    active_jwt_authority_id varchar,
    PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS federated_trust_domains (
    id int,
    created_at timestamp,
    updated_at timestamp,
    trust_domain varchar,
    bundle_endpoint_url varchar,
    bundle_endpoint_profile varchar,
    endpoint_spiffe_id varchar,
    implicit boolean,
    PRIMARY KEY (trust_domain)
);

CREATE TABLE IF NOT EXISTS join_tokens (
    id int,
    created_at timestamp,
    updated_at timestamp,
    join_token varchar,
    expiry bigint,
    PRIMARY KEY (join_token)
);

CREATE TABLE IF NOT EXISTS node_resolver_map_entries (
    id int,
    created_at timestamp,
    updated_at timestamp,
    spiffe_id varchar,
    type varchar,
    value varchar,
    PRIMARY KEY ((spiffe_id, type, value))
);