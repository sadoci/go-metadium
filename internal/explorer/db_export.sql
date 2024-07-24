CREATE TABLE IF NOT EXISTS block_data
(
    number      BIGINT NOT NULL,
    hash        VARCHAR(66) NOT NULL,
    block_data  MEDIUMTEXT NOT NULL,
    trace_data  MEDIUMTEXT,

    PRIMARY KEY (hash)
);

-- CREATE INDEX IF NOT EXISTS idx_block_data_number ON block_data (number ASC);
CREATE INDEX idx_block_data_number ON block_data (number ASC);
