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

package ipfsethdb

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ipfs/boxo/blockservice"
)

var (
	stateTrieCodec       uint64 = 0x96
	defaultBatchCapacity        = 1024
	errNotSupported             = errors.New("this operation is not supported")
)

var _ ethdb.Database = &Database{}

// Database is the type that satisfies the ethdb.Database and ethdb.KeyValueStore interfaces for IPFS Ethereum data
// This is ipfs-backing-datastore agnostic but must operate through a configured ipfs node (and so is subject to lockfile contention with e.g. an ipfs daemon)
// If blockservice block exchange is configured the blockservice can fetch data that are missing locally from IPFS peers
type Database struct {
	blockService blockservice.BlockService
}

// NewKeyValueStore returns a ethdb.KeyValueStore interface for IPFS
func NewKeyValueStore(bs blockservice.BlockService) ethdb.KeyValueStore {
	return &Database{
		blockService: bs,
	}
}

// NewDatabase returns a ethdb.Database interface for IPFS
func NewDatabase(bs blockservice.BlockService) ethdb.Database {
	return &Database{
		blockService: bs,
	}
}

func (d *Database) ModifyAncients(f func(ethdb.AncientWriteOp) error) (int64, error) {
	return 0, errNotSupported
}

// Has satisfies the ethdb.KeyValueReader interface
// Has retrieves if a key is present in the key-value data store
// This only operates on the local blockstore not through the exchange
func (d *Database) Has(key []byte) (bool, error) {
	// we are using state codec because we don't know the codec and at this level the codec doesn't matter, the datastore key is multihash-only derived
	c, err := Keccak256ToCid(key, stateTrieCodec)
	if err != nil {
		return false, err
	}
	return d.blockService.Blockstore().Has(context.Background(), c)
}

// Get satisfies the ethdb.KeyValueReader interface
// Get retrieves the given key if it's present in the key-value data store
func (d *Database) Get(key []byte) ([]byte, error) {
	// we are using state codec because we don't know the codec and at this level the codec doesn't matter, the datastore key is multihash-only derived
	c, err := Keccak256ToCid(key, stateTrieCodec)
	if err != nil {
		return nil, err
	}
	block, err := d.blockService.GetBlock(context.Background(), c)
	if err != nil {
		return nil, err
	}
	return block.RawData(), nil
}

// Put satisfies the ethdb.KeyValueWriter interface
// Put inserts the given value into the key-value data store
// Key is expected to be the keccak256 hash of value
func (d *Database) Put(key []byte, value []byte) error {
	b, err := NewBlock(key, value)
	if err != nil {
		return err
	}
	return d.blockService.AddBlock(context.Background(), b)
}

// Delete satisfies the ethdb.KeyValueWriter interface
// Delete removes the key from the key-value data store
func (d *Database) Delete(key []byte) error {
	// we are using state codec because we don't know the codec and at this level the codec doesn't matter, the datastore key is multihash-only derived
	c, err := Keccak256ToCid(key, stateTrieCodec)
	if err != nil {
		return err
	}
	return d.blockService.DeleteBlock(context.Background(), c)
}

// DatabaseProperty enum type
type DatabaseProperty int

const (
	Unknown DatabaseProperty = iota
	ExchangeOnline
)

// DatabasePropertyFromString helper function
func DatabasePropertyFromString(property string) (DatabaseProperty, error) {
	switch strings.ToLower(property) {
	case "exchange", "online":
		return ExchangeOnline, nil
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
	b, err := NewBatch(d.blockService, defaultBatchCapacity)
	if err != nil {
		panic(err)
	}
	return b
}

// NewBatchWithSize satisfies the ethdb.Batcher interface.
// NewBatchWithSize creates a write-only database batch with pre-allocated buffer.
func (d *Database) NewBatchWithSize(size int) ethdb.Batch {
	b, err := NewBatch(d.blockService, size)
	if err != nil {
		panic(err)
	}
	return b
}

// NewIterator satisfies the ethdb.Iteratee interface
// it creates a binary-alphabetical iterator over a subset
// of database content with a particular key prefix, starting at a particular
// initial key (or after, if it does not exist).
//
// Note: This method assumes that the prefix is NOT part of the start, so there's
// no need for the caller to prepend the prefix to the start
func (d *Database) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	return NewIterator(start, prefix, d.blockService)
}

// Close satisfies the io.Closer interface
// Close closes the db connection
func (d *Database) Close() error {
	return d.blockService.Close()
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
