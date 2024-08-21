CREATE TABLE IF NOT EXISTS trades (
    amm_id VARCHAR(255),
    mint VARCHAR(255),
    action VARCHAR(255),
    amount BIGINT,
    signature VARCHAR(255),
    timestamp INT
);
