// VulcanizeDB
// Copyright © 2020 Vulcanize

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.

// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package pgipfsethdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ipfs/go-cid"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/jmoiron/sqlx"
	"github.com/mailgun/groupcache/v2"
	log "github.com/sirupsen/logrus"
)

var errNotSupported = errors.New("this operation is not supported")

var (
	hasPgStr    = "SELECT exists(select 1 from ipld.blocks WHERE key = $1 LIMIT 1)"
	getPgStr    = "SELECT data FROM ipld.blocks WHERE key = $1 LIMIT 1"
	putPgStr    = "INSERT INTO ipld.blocks (key, data, block_number) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING"
	deletePgStr = "DELETE FROM ipld.blocks WHERE key = $1"
	dbSizePgStr = "SELECT pg_database_size(current_database())"
)

var _ ethdb.Database = &Database{}

// Database is the type that satisfies the ethdb.Database and ethdb.KeyValueStore interfaces for PG-IPFS Ethereum data using a direct Postgres connection
type Database struct {
	db    *sqlx.DB
	cache *groupcache.Group

	BlockNumber *big.Int
}

func (d *Database) ModifyAncients(f func(ethdb.AncientWriteOp) error) (int64, error) {
	return 0, errNotSupported
}

type CacheConfig struct {
	Name           string
	Size           int
	ExpiryDuration time.Duration
}

// NewKeyValueStore returns a ethdb.KeyValueStore interface for PG-IPFS
func NewKeyValueStore(db *sqlx.DB, cacheConfig CacheConfig) ethdb.KeyValueStore {
	database := Database{db: db}
	database.InitCache(cacheConfig)

	return &database
}

// NewDatabase returns a ethdb.Database interface for PG-IPFS
func NewDatabase(db *sqlx.DB, cacheConfig CacheConfig) ethdb.Database {
	database := Database{db: db}
	database.InitCache(cacheConfig)

	return &database
}

func (d *Database) InitCache(cacheConfig CacheConfig) {
	d.cache = groupcache.NewGroup(cacheConfig.Name, int64(cacheConfig.Size), groupcache.GetterFunc(
		func(_ context.Context, id string, dest groupcache.Sink) error {
			val, err := d.dbGet(id)

			if err != nil {
				return err
			}

			// Set the value in the groupcache, with expiry
			if err := dest.SetBytes(val, time.Now().Add(cacheConfig.ExpiryDuration)); err != nil {
				return err
			}

			return nil
		},
	))
}

func (d *Database) GetCacheStats() groupcache.Stats {
	return d.cache.Stats
}

// Has satisfies the ethdb.KeyValueReader interface
// Has retrieves if a cid is present in the key-value data store
func (d *Database) Has(cidBytes []byte) (bool, error) {
	c, err := cid.Cast(cidBytes)
	if err != nil {
		return false, err
	}
	var exists bool
	return exists, d.db.Get(&exists, hasPgStr, c.String())
}

// Get retrieves the given key if it's present in the key-value data store
func (d *Database) dbGet(key string) ([]byte, error) {
	var data []byte
	err := d.db.Get(&data, getPgStr, key)
	if err == sql.ErrNoRows {
		log.Warn("Database miss for key ", key)
	}

	return data, err
}

// Get satisfies the ethdb.KeyValueReader interface
// Get retrieves the given cid if it's present in the key-value data store
func (d *Database) Get(cidBytes []byte) ([]byte, error) {
	c, err := cid.Cast(cidBytes)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*500)
	defer cancel()

	var data []byte
	return data, d.cache.Get(ctx, c.String(), groupcache.AllocatingByteSliceSink(&data))
}

// Put satisfies the ethdb.KeyValueWriter interface
// Put inserts the given value into the key-value data store
// Key is expected to be a fully formulated cis of value
func (d *Database) Put(cidBytes []byte, value []byte) error {
	c, err := cid.Cast(cidBytes)
	if err != nil {
		return err
	}
	_, err = d.db.Exec(putPgStr, c.String(), value, d.BlockNumber.Uint64())
	return err
}

// Delete satisfies the ethdb.KeyValueWriter interface
// Delete removes the cid from the key-value data store
func (d *Database) Delete(cidBytes []byte) error {
	c, err := cid.Cast(cidBytes)
	if err != nil {
		return err
	}
	cidString := c.String()

	_, err = d.db.Exec(deletePgStr, cidString)
	if err != nil {
		return err
	}

	// Remove from cache.
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*500)
	defer cancel()
	err = d.cache.Remove(ctx, cidString)

	return err
}

// DatabaseProperty enum type
type DatabaseProperty int

const (
	Unknown DatabaseProperty = iota
	Size
	Idle
	InUse
	MaxIdleClosed
	MaxLifetimeClosed
	MaxOpenConnections
	OpenConnections
	WaitCount
	WaitDuration
)

// DatabasePropertyFromString helper function
func DatabasePropertyFromString(property string) (DatabaseProperty, error) {
	switch strings.ToLower(property) {
	case "size":
		return Size, nil
	case "idle":
		return Idle, nil
	case "inuse":
		return InUse, nil
	case "maxidleclosed":
		return MaxIdleClosed, nil
	case "maxlifetimeclosed":
		return MaxLifetimeClosed, nil
	case "maxopenconnections":
		return MaxOpenConnections, nil
	case "openconnections":
		return OpenConnections, nil
	case "waitcount":
		return WaitCount, nil
	case "waitduration":
		return WaitDuration, nil
	default:
		return Unknown, fmt.Errorf("unknown database property")
	}
}

// Stat satisfies the ethdb.Stater interface
// Stat returns a particular internal stat of the database
func (d *Database) Stat(property string) (string, error) {
	prop, err := DatabasePropertyFromString(property)
	if err != nil {
		return "", err
	}
	switch prop {
	case Size:
		var byteSize string
		return byteSize, d.db.Get(&byteSize, dbSizePgStr)
	case Idle:
		return strconv.Itoa(d.db.Stats().Idle), nil
	case InUse:
		return strconv.Itoa(d.db.Stats().InUse), nil
	case MaxIdleClosed:
		return strconv.FormatInt(d.db.Stats().MaxIdleClosed, 10), nil
	case MaxLifetimeClosed:
		return strconv.FormatInt(d.db.Stats().MaxLifetimeClosed, 10), nil
	case MaxOpenConnections:
		return strconv.Itoa(d.db.Stats().MaxOpenConnections), nil
	case OpenConnections:
		return strconv.Itoa(d.db.Stats().OpenConnections), nil
	case WaitCount:
		return strconv.FormatInt(d.db.Stats().WaitCount, 10), nil
	case WaitDuration:
		return d.db.Stats().WaitDuration.String(), nil
	default:
		return "", fmt.Errorf("unhandled database property")
	}
}

// Compact satisfies the ethdb.Compacter interface
// Compact flattens the underlying data store for the given key range
func (d *Database) Compact(start []byte, limit []byte) error {
	return errNotSupported
}

// NewBatch satisfies the ethdb.Batcher interface
// NewBatch creates a write-only database that buffers changes to its host db
// until a final write is called
func (d *Database) NewBatch() ethdb.Batch {
	return NewBatch(d.db, nil, d.BlockNumber)
}

// NewBatchWithSize satisfies the ethdb.Batcher interface.
// NewBatchWithSize creates a write-only database batch with pre-allocated buffer.
func (d *Database) NewBatchWithSize(size int) ethdb.Batch {
	return NewBatch(d.db, nil, d.BlockNumber)
}

// NewIterator satisfies the ethdb.Iteratee interface
// it creates a binary-alphabetical iterator over a subset
// of database content with a particular key prefix, starting at a particular
// initial key (or after, if it does not exist).
//
// Note: This method assumes that the prefix is NOT part of the start, so there's
// no need for the caller to prepend the prefix to the start
func (d *Database) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	return NewIterator(start, prefix, d.db)
}

// Close satisfies the io.Closer interface.
// Close closes the db connection and deregisters from groupcache.
func (d *Database) Close() error {
	groupcache.DeregisterGroup(d.cache.Name())
	return d.db.DB.Close()
}

// HasAncient satisfies the ethdb.AncientReader interface
// HasAncient returns an indicator whether the specified data exists in the ancient store
func (d *Database) HasAncient(kind string, number uint64) (bool, error) {
	return false, errNotSupported
}

// Ancient satisfies the ethdb.AncientReader interface
// Ancient retrieves an ancient binary blob from the append-only immutable files
func (d *Database) Ancient(kind string, number uint64) ([]byte, error) {
	return nil, errNotSupported
}

// Ancients satisfies the ethdb.AncientReader interface
// Ancients returns the ancient item numbers in the ancient store
func (d *Database) Ancients() (uint64, error) {
	return 0, errNotSupported
}

// Tail satisfies the ethdb.AncientReader interface.
// Tail returns the number of first stored item in the freezer.
func (d *Database) Tail() (uint64, error) {
	return 0, errNotSupported
}

// AncientSize satisfies the ethdb.AncientReader interface
// AncientSize returns the ancient size of the specified category
func (d *Database) AncientSize(kind string) (uint64, error) {
	return 0, errNotSupported
}

// AncientRange retrieves all the items in a range, starting from the index 'start'.
// It will return
//   - at most 'count' items,
//   - at least 1 item (even if exceeding the maxBytes), but will otherwise
//     return as many items as fit into maxBytes.
func (d *Database) AncientRange(kind string, start, count, maxBytes uint64) ([][]byte, error) {
	return nil, errNotSupported
}

// ReadAncients applies the provided AncientReader function
func (d *Database) ReadAncients(fn func(ethdb.AncientReaderOp) error) (err error) {
	return errNotSupported
}

// TruncateHead satisfies the ethdb.AncientWriter interface.
// TruncateHead discards all but the first n ancient data from the ancient store.
func (d *Database) TruncateHead(n uint64) (uint64, error) {
	return 0, errNotSupported
}

// TruncateTail satisfies the ethdb.AncientWriter interface.
// TruncateTail discards the first n ancient data from the ancient store.
func (d *Database) TruncateTail(n uint64) (uint64, error) {
	return 0, errNotSupported
}

// Sync satisfies the ethdb.AncientWriter interface
// Sync flushes all in-memory ancient store data to disk
func (d *Database) Sync() error {
	return errNotSupported
}

// MigrateTable satisfies the ethdb.AncientWriter interface.
// MigrateTable processes and migrates entries of a given table to a new format.
func (d *Database) MigrateTable(string, func([]byte) ([]byte, error)) error {
	return errNotSupported
}

// NewSnapshot satisfies the ethdb.Snapshotter interface.
// NewSnapshot creates a database snapshot based on the current state.
func (d *Database) NewSnapshot() (ethdb.Snapshot, error) {
	return nil, errNotSupported
}

// AncientDatadir returns an error as we don't have a backing chain freezer.
func (d *Database) AncientDatadir() (string, error) {
	return "", errNotSupported
}
