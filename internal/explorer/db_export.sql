CREATE TABLE IF NOT EXISTS block_data
(
    number      BIGINT NOT NULL,
    hash        VARCHAR(66) NOT NULL,
    block_data  TEXT NOT NULL,
    trace_data  TEXT,

    PRIMARY KEY (hash)
);

CREATE INDEX IF NOT EXISTS idx_block_data_number ON block_data (number ASC);