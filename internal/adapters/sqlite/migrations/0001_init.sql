CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS domains (
    id                   TEXT PRIMARY KEY,
    name                 TEXT NOT NULL UNIQUE,
    challenge_type       TEXT NOT NULL,
    dns_provider         TEXT NOT NULL DEFAULT '',
    encrypted_credential BLOB,
    k8s_namespace        TEXT NOT NULL,
    k8s_secret_name      TEXT NOT NULL,
    auto_renew           INTEGER NOT NULL DEFAULT 0,
    renew_before_days    INTEGER NOT NULL DEFAULT 30,
    status               TEXT NOT NULL DEFAULT 'pending',
    last_error           TEXT NOT NULL DEFAULT '',
    created_at           DATETIME NOT NULL,
    updated_at           DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS certificates (
    id                TEXT PRIMARY KEY,
    domain_id         TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    issued_at         DATETIME NOT NULL,
    expires_at        DATETIME NOT NULL,
    cert_pem          BLOB NOT NULL,
    chain_pem         BLOB NOT NULL,
    encrypted_key_pem BLOB NOT NULL,
    status            TEXT NOT NULL,
    created_at        DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_certificates_domain_id ON certificates(domain_id);

CREATE TABLE IF NOT EXISTS renewal_jobs (
    id          TEXT PRIMARY KEY,
    domain_id   TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    trigger     TEXT NOT NULL,
    status      TEXT NOT NULL,
    message     TEXT NOT NULL DEFAULT '',
    started_at  DATETIME NOT NULL,
    finished_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_renewal_jobs_domain_id ON renewal_jobs(domain_id);
