CREATE TABLE IF NOT EXISTS registered_entries (
    id int,
    created_at timestamp,
    updated_at timestamp,
    entry_id varchar,
    spiffe_id varchar,
    parent_id varchar,
    ttl int,
    admin boolean,
    downstream boolean,
    expiry bigint,
    revision_number bigint,
    store_svid boolean,
    hint varchar,
    jwt_svid_ttl int,
    PRIMARY KEY (entry_id)
);

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
    id int, 
    created_at timestamp,
    updated_at timestamp,
    trust_domain varchar,
    data blob,
    PRIMARY KEY (trust_domain, id)
);

CREATE TABLE IF NOT EXISTS ca_journals (
    id int,
    created_at timestamp,
    updated_at timestamp,
    data blob,
    active_x509_authority_id varchar,
    active_jwt_authority_id varchar,
    PRIMARY KEY (id)
);

--- TODO(tjons): this should _absolutely_ be folded into the registered_entries table
CREATE TABLE IF NOT EXISTS dns_names (
    id int,
    created_at timestamp,
    updated_at timestamp,
    registered_entry_id int,
    value varchar,
    PRIMARY KEY(registered_entry_id, value)
);

--- TODO(tjons): good case for consolidation...
CREATE TALBE IF NOT EXISTS federated_registration_entries (
    bundle_id int,
    registered_entry_id int,
    PRIMARY KEY (registered_entry_id, bundle_id)
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