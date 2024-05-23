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

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
)

var (
	blockNotFoundErr = errors.New("block not found")
)

type MockBlockservice struct {
	blockStore *MockBlockstore
	err        error
}

func NewMockBlockservice() blockservice.BlockService {
	return &MockBlockservice{
		blockStore: &MockBlockstore{
			blocks: make(map[string]blocks.Block),
		},
	}
}

func (mbs *MockBlockservice) Blockstore() blockstore.Blockstore {
	return mbs.blockStore
}

func (mbs *MockBlockservice) Exchange() exchange.Interface {
	panic("Exchange: implement me")
}

func (mbs *MockBlockservice) AddBlock(ctx context.Context, b blocks.Block) error {
	return mbs.blockStore.Put(ctx, b)
}

func (mbs *MockBlockservice) AddBlocks(ctx context.Context, bs []blocks.Block) error {
	return mbs.blockStore.PutMany(ctx, bs)
}

func (mbs *MockBlockservice) DeleteBlock(ctx context.Context, c cid.Cid) error {
	return mbs.blockStore.DeleteBlock(ctx, c)
}

func (mbs *MockBlockservice) GetBlock(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	return mbs.blockStore.Get(ctx, c)
}

func (mbs *MockBlockservice) GetBlocks(ctx context.Context, cs []cid.Cid) <-chan blocks.Block {
	blockChan := make(chan blocks.Block)
	go func() {
		for _, c := range cs {
			if b, err := mbs.blockStore.Get(ctx, c); err == nil {
				blockChan <- b
			}
		}
	}()
	return blockChan
}

func (mbs *MockBlockservice) Close() error {
	return mbs.err
}

func (mbs *MockBlockservice) SetError(err error) {
	mbs.err = err
}

type MockBlockstore struct {
	blocks map[string]blocks.Block
	err    error
}

func (mbs *MockBlockstore) DeleteBlock(ctx context.Context, c cid.Cid) error {
	delete(mbs.blocks, c.String())
	return mbs.err
}

func (mbs *MockBlockstore) Has(ctx context.Context, c cid.Cid) (bool, error) {
	_, ok := mbs.blocks[c.String()]
	return ok, mbs.err
}

func (mbs *MockBlockstore) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	obj, ok := mbs.blocks[c.String()]
	if !ok {
		return nil, blockNotFoundErr
	}
	return obj, mbs.err
}

func (mbs *MockBlockstore) GetSize(ctx context.Context, c cid.Cid) (int, error) {
	obj, ok := mbs.blocks[c.String()]
	if !ok {
		return 0, blockNotFoundErr
	}
	return len(obj.RawData()), mbs.err
}

func (mbs *MockBlockstore) Put(ctx context.Context, b blocks.Block) error {
	mbs.blocks[b.Cid().String()] = b
	return mbs.err
}

func (mbs *MockBlockstore) PutMany(ctx context.Context, bs []blocks.Block) error {
	for _, b := range bs {
		mbs.blocks[b.Cid().String()] = b
	}
	return mbs.err
}

func (mbs *MockBlockstore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	panic("AllKeysChan: implement me")
}

func (mbs *MockBlockstore) HashOnRead(enabled bool) {
	panic("HasOnRead: implement me")
}

func (mbs *MockBlockstore) SetError(err error) {
	mbs.err = err
}
