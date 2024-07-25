CREATE TABLE IF NOT EXISTS block_data
(
    number      BIGINT NOT NULL,
    hash        VARCHAR(66) NOT NULL,
    block_data  MEDIUMTEXT NOT NULL,
    trace_data  MEDIUMTEXT,

    PRIMARY KEY (hash)
);

CREATE TABLE IF NOT EXISTS internal_transactions
(
    tx_hash      VARCHAR(66) NOT NULL,
    block_hash   VARCHAR(66) NOT NULL,
    block_number BIGINT NOT NULL,
    tx_index     INT NOT NULL,
    call_index   INT NOT NULL,
    `from`       VARCHAR(42) NOT NULL,
    `to`         VARCHAR(42) NOT NULL,
    value        VARCHAR(66) NOT NULL,

    PRIMARY KEY (tx_hash, call_index)
);

-- CREATE INDEX IF NOT EXISTS idx_block_data_number ON block_data (number ASC);
CREATE INDEX idx_block_data_number ON block_data (number ASC);
