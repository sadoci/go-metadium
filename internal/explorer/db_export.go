package explorer

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/log"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

var (
	explorerDb  *sql.DB
	blockTracer *tracers.Tracer
)

func blockImportHook(bc *core.BlockChain, block *types.Block, receipts types.Receipts, traceData []byte) map[string]interface{} {
	blockData := ethapi.RPCMarshalBlockEx(bc, block, receipts, traceData)
	jsonData, err := json.Marshal(blockData)
	if err != nil {
		log.Error("Failed to marshal block data", "number", block.Number(), "hash", block.Hash(), "err", err)
		return nil
	}
	// TODO: ? -> $X
	if len(traceData) != 0 {
		_, err = explorerDb.Exec("INSERT INTO block_data (number, hash, block_data, trace_data) VALUES (?, ?, ?, ?)",
			block.Number().Int64(), block.Hash().Hex(), string(jsonData), string(traceData))
	} else {
		_, err = explorerDb.Exec("INSERT INTO block_data (number, hash, block_data) VALUES (?, ?, ?)",
			block.Number().Int64(), block.Hash().Hex(), string(jsonData))
	}
	if err != nil {
		log.Error("Failed to insert block data", "number", block.Number(), "hash", block.Hash(), "err", err)
	}

	return nil
}

// "mysql username:password@tcp(127.0.0.1:3306)/dbname"
// "postgresql user=username password=password dbname=dbname host=127.0.0.1 port=5432 sslmode=disable"

func SetupExplorerDB(dbParams string) {
	if len(dbParams) == 0 {
		return
	}

	if file, err := os.Open(dbParams); err != nil {
		panic(err)
	} else {
		buf := &bytes.Buffer{}
		_, err := io.Copy(buf, file)
		if err != nil {
			panic(err)
		}
		parts := strings.SplitN(buf.String(), " ", 2)
		if len(parts) < 2 {
			panic("ExplorerDBParams is invalid")
		}
		prefix := strings.TrimSpace(parts[0])
		params := strings.TrimSpace(parts[1])
		if strings.HasPrefix(prefix, "postgres") {
			explorerDb, err = sql.Open("postgres", params)
			if err != nil {
				panic(err)
			}
			if err = explorerDb.Ping(); err != nil {
				panic(err)
			}
		} else if strings.HasPrefix(prefix, "mysql") {
			explorerDb, err = sql.Open("mysql", params)
			if err != nil {
				panic(err)
			}
			if err = explorerDb.Ping(); err != nil {
				panic(err)
			}
		} else {
			panic("ExplorerDBParams is neither postgresql or mysql")
		}
	}

	core.BlockImportHook = blockImportHook
	core.BlockTraceSetup = func(c *vm.Config) {
		if blockTracer == nil {
			tracer, err := tracers.DefaultDirectory.New("blockCallsTracer", new(tracers.Context), nil)
			if err != nil {
				panic(err)
			}
			blockTracer = tracer
		}
		c.Tracer = blockTracer.Hooks
	}
	core.BlockTraceGetResult = func(c *vm.Config) ([]byte, error) {
		if blockTracer == nil {
			panic("blockTracer is not set up.")
		}
		res, err := blockTracer.GetResult()
		if err != nil {
			return nil, err
		}
		return res, nil
	}
}
