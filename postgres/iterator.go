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
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

const (
	nextPgStr = `SELECT key, data FROM public.blocks
				INNER JOIN eth.key_preimages ON (ipfs_key = key)
				WHERE eth_key > $1 ORDER BY eth_key LIMIT 1`
)

type nextModel struct {
	Key   []byte `db:"eth_key"`
	Value []byte `db:"data"`
}

// Iterator is the type that satisfies the ethdb.Iterator interface for PG-IPFS Ethereum data using a direct Postgres connection
type Iterator struct {
	db                               *sqlx.DB
	currentKey, prefix, currentValue []byte
	err                              error
}

// NewIterator returns an ethdb.Iterator interface for PG-IPFS
func NewIterator(start, prefix []byte, db *sqlx.DB) ethdb.Iterator {
	return &Iterator{
		db:         db,
		prefix:     prefix,
		currentKey: start,
	}
}

// Next satisfies the ethdb.Iterator interface
// Next moves the iterator to the next key/value pair
// It returns whether the iterator is exhausted
func (i *Iterator) Next() bool {
	next := new(nextModel)
	if err := i.db.Get(next, nextPgStr, i.currentKey); err != nil {
		logrus.Errorf("iterator.Next() error: %v", err)
		i.currentKey, i.currentValue = nil, nil
		return false
	}
	i.currentKey, i.currentValue = next.Key, next.Value
	return true
}

// Error satisfies the ethdb.Iterator interface
// Error returns any accumulated error
// Exhausting all the key/value pairs is not considered to be an error
func (i *Iterator) Error() error {
	return i.err
}

// Key satisfies the ethdb.Iterator interface
// Key returns the key of the current key/value pair, or nil if done
// The caller should not modify the contents of the returned slice
// and its contents may change on the next call to Next
func (i *Iterator) Key() []byte {
	return i.currentKey
}

// Value satisfies the ethdb.Iterator interface
// Value returns the value of the current key/value pair, or nil if done
// The caller should not modify the contents of the returned slice
// and its contents may change on the next call to Next
func (i *Iterator) Value() []byte {
	return i.currentValue
}

// Release satisfies the ethdb.Iterator interface
// Release releases associated resources
// Release should always succeed and can be called multiple times without causing error
func (i *Iterator) Release() {
	i.db.Close()
}
