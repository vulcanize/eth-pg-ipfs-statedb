## ipfs-ethdb

[![Go Report Card](https://goreportcard.com/badge/github.com/cerc/ipfs-ethdb)](https://goreportcard.com/report/github.com/cerc-io/ipfs-ethdb)

> go-ethereum ethdb interfaces for Ethereum state data stored in IPFS

## Background

Go-ethereum defines a number of interfaces in the [ethdb package](https://github.com/ethereum/go-ethereum/tree/master/ethdb) for
interfacing with a state database. These interfaces are used to build higher-level types such as the [trie.Database](https://github.com/ethereum/go-ethereum/blob/master/trie/database.go#L77)
which are used to perform the bulk of state related needs.

Ethereum data can be stored on IPFS, standard codecs for Etheruem data are defined in the [go-cid](https://github.com/ipfs/go-cid) library.
Using our [statediffing geth client](https://github.com/cerc-io/go-ethereum/releases/tag/v1.9.11-statediff-0.0.2) it is feasible to extract every single
state and storage node and publish it to IPFS.

Geth stores state data in leveldb as key-value pairs between the keccak256 hash of the rlp-encoded object and the rlp-encoded object.
Ethereum data on IPFS is also stored as key-value pairs with the value being the rlp-encoded byte value for the object,
but the key is a derivative of the keccak256 hash rather than the hash itself. This library provides
ethdb interfaces for Ethereum data on IPFS by handling the conversion of a keccak256 hash to its multihash-derived key.


## Usage
To use this module import it and build an ethdb interface around an instance of a [Go IPFS blockservice](https://pkg.go.dev/github.com/ipfs/boxo@v0.19.0/blockservice), you can then
employ it as you would the usual [leveldb](https://github.com/ethereum/go-ethereum/tree/master/ethdb/leveldb) or [memorydb](https://github.com/ethereum/go-ethereum/tree/master/ethdb/memorydb) ethdbs
with some exceptions: the AncientReader, AncientWriter, Compacter, and Iteratee/Iterator interfaces are not functionally complete.

Ancient data does not currently have a representation on IPFS, and recapitulation of the database key iterator and compacter is complicated since go-ethereum
types that use this interface expect the iterator and compacter to operate over keccak256 hash key ranges, whereas the keys for Ethereum data on IPFS are derived from that hash but not the hash itself.

Iteratee interface is used in Geth for various tests, in trie/sync_bloom.go (for fast sync), rawdb.InspectDatabase, and the new (1.9.15) core/state/snapshot features;
Ancient interfaces are used for Ancient/frozen data operations (e.g. rawdb/table.go); and Compacter is used in core/state/snapshot, rawdb/table.go, chaincmd.go, and the private debug api.

Outside of these primarily auxiliary capabilities, this package satisfies the interfaces required for many state operations using Ethereum data on IPFS.

e.g.

go-ethereum trie.NodeIterator and state.NodeIterator can be constructed from the ethdb.KeyValueStore and ethdb.Database interfaces, respectively:

```go
package main

import (
    "context"

    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/core/state"
    "github.com/ethereum/go-ethereum/trie"
    "github.com/ipfs/boxo/blockservice"
    "github.com/ipfs/go-ipfs/core"
    "github.com/ipfs/go-ipfs/repo/fsrepo"
    "github.com/jmoiron/sqlx"
    "github.com/cerc-io/ipfs-ethdb/v5"
)

func main() {
    // errors are ignored for brevity
    ipfsPath := "~/.ipfs"
    r, _ := fsrepo.Open(ipfsPath)
    ctx := context.Background()
    cfg := &core.BuildCfg{
        Online: false,
        Repo:   r,
    }
    ipfsNode, _ := core.NewNode(ctx, cfg)
    kvs := ipfsethdb.NewKeyValueStore(ipfsNode.Blocks)
    trieDB := trie.NewDatabase(kvs)
    t, _ := trie.New(common.Hash{}, trieDB)
    trieNodeIterator := t.NodeIterator([]byte{})
    // do stuff with trie node iterator

    database := ipfsethdb.NewDatabase(ipfsNode.Blocks)
    stateDatabase := state.NewDatabase(database)
    stateDB, _ := state.New(common.Hash{}, stateDatabase, nil)
    stateDBNodeIterator := state.NewNodeIterator(stateDB)
    // do stuff with the statedb node iterator
}
```

[Types are also available](./postgres/doc.md) for interfacing directly with the data stored in an IPFS-backing Postgres database

## Maintainers
@vulcanize
@AFDudley
@i-norden

## Contributing
Contributions are welcome!

VulcanizeDB follows the [Contributor Covenant Code of Conduct](https://www.contributor-covenant.org/version/1/4/code-of-conduct).

## License
[AGPL-3.0](LICENSE) © Vulcanize Inc
