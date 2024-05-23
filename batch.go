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

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	lru "github.com/hashicorp/golang-lru"
	"github.com/ipfs/boxo/blockservice"
	blocks "github.com/ipfs/go-block-format"
)

var (
	EvictionWarningErr = errors.New("warn: batch has exceeded capacity, data has been evicted")
)

var _ ethdb.Batch = &Batch{}

// Batch is the type that satisfies the ethdb.Batch interface for IPFS Ethereum data using the ipfs blockservice interface
// This is ipfs-backing-datastore agnostic but must operate through a configured ipfs node (and so is subject to lockfile contention with e.g. an ipfs daemon)
// If blockservice block exchange is configured the blockservice can fetch data that are missing locally from IPFS peers
type Batch struct {
	blockService          blockservice.BlockService
	putCache, deleteCache *lru.Cache
	valueSize             int
}

// NewBatch returns a ethdb.Batch interface for IPFS
func NewBatch(bs blockservice.BlockService, capacity int) (ethdb.Batch, error) {
	putCache, err := lru.New(capacity)
	if err != nil {
		return nil, err
	}
	deleteCache, err := lru.New(capacity)
	if err != nil {
		return nil, err
	}
	return &Batch{
		blockService: bs,
		putCache:     putCache,
		deleteCache:  deleteCache,
	}, nil
}

// Put satisfies the ethdb.Batch interface
// Put inserts the given value into the key-value data store
// Key is expected to be the keccak256 hash of value
// Returns an error when batch capacity has been exceeded and data was evicted
// It is up to ensure they do not exceed capacity
// The alternative is to check the cache len vs its capacity before inserting
// but this adds additional overhead to every Put/Delete
func (b *Batch) Put(key []byte, value []byte) (err error) {
	b.valueSize += len(value)
	strKey := common.Bytes2Hex(key)
	if b.putCache.Add(strKey, value) {
		return EvictionWarningErr
	}
	return nil
}

// Delete satisfies the ethdb.Batch interface
// Delete removes the key from the key-value data store
func (b *Batch) Delete(key []byte) (err error) {
	strKey := common.Bytes2Hex(key)
	if b.deleteCache.Add(strKey, true) {
		return EvictionWarningErr
	}
	return nil
}

// ValueSize satisfies the ethdb.Batch interface
// ValueSize retrieves the amount of data queued up for writing
// The returned value is the total byte length of all data queued to write
func (b *Batch) ValueSize() int {
	return b.valueSize
}

// Write satisfies the ethdb.Batch interface
// Write flushes any accumulated data to disk
func (b *Batch) Write() error {
	puts := make([]blocks.Block, b.putCache.Len())
	for i, key := range b.putCache.Keys() {
		val, _ := b.putCache.Get(key) // don't need to check "ok"s, the key is known and val is always []byte
		b, err := NewBlock(common.Hex2Bytes(key.(string)), val.([]byte))
		if err != nil {
			return err
		}
		puts[i] = b
	}
	if err := b.blockService.AddBlocks(context.Background(), puts); err != nil {
		return err
	}
	for _, key := range b.deleteCache.Keys() {
		// we are using state codec because we don't know the codec and at this level the codec doesn't matter, the datastore key is multihash-only derived
		c, err := Keccak256ToCid(common.Hex2Bytes(key.(string)), stateTrieCodec)
		if err != nil {
			return err
		}
		if err := b.blockService.DeleteBlock(context.Background(), c); err != nil {
			return err
		}
	}
	return nil
}

// Replay satisfies the ethdb.Batch interface
// Replay replays the batch contents
func (b *Batch) Replay(w ethdb.KeyValueWriter) error {
	for _, key := range b.putCache.Keys() {
		val, _ := b.putCache.Get(key)
		if err := w.Put(key.([]byte), val.([]byte)); err != nil {
			return err
		}
	}
	for _, key := range b.deleteCache.Keys() {
		if err := w.Delete(key.([]byte)); err != nil {
			return err
		}
	}
	return nil
}

// Reset satisfies the ethdb.Batch interface
// Reset resets the batch for reuse
// This should be called after every write
func (b *Batch) Reset() {
	b.deleteCache.Purge()
	b.putCache.Purge()
	b.valueSize = 0
}
