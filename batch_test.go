// VulcanizeDB
// Copyright © 2019 Vulcanize

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

package ipfsethdb_test

import (
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ipfsethdb "github.com/cerc-io/ipfs-ethdb/v5"
)

var (
	batch         ethdb.Batch
	testHeader2   = types.Header{Number: big.NewInt(2)}
	testValue2, _ = rlp.EncodeToBytes(testHeader2)
	testEthKey2   = testHeader2.Hash().Bytes()
)

var _ = Describe("Batch", func() {
	BeforeEach(func() {
		blockService = ipfsethdb.NewMockBlockservice()
		batch, err = ipfsethdb.NewBatch(blockService, 1024)
		Expect(err).ToNot(HaveOccurred())
		database = ipfsethdb.NewDatabase(blockService)
	})

	Describe("Put/Write", func() {
		It("adds the key-value pair to the batch", func() {
			_, err = database.Get(testEthKey)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("block not found"))
			_, err = database.Get(testEthKey2)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("block not found"))

			err = batch.Put(testEthKey, testValue)
			Expect(err).ToNot(HaveOccurred())
			err = batch.Put(testEthKey2, testValue2)
			Expect(err).ToNot(HaveOccurred())
			err = batch.Write()
			Expect(err).ToNot(HaveOccurred())

			val, err := database.Get(testEthKey)
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(Equal(testValue))
			val2, err := database.Get(testEthKey2)
			Expect(err).ToNot(HaveOccurred())
			Expect(val2).To(Equal(testValue2))
		})
	})

	Describe("Delete/Reset/Write", func() {
		It("deletes the key-value pair in the batch", func() {
			err = batch.Put(testEthKey, testValue)
			Expect(err).ToNot(HaveOccurred())
			err = batch.Put(testEthKey2, testValue2)
			Expect(err).ToNot(HaveOccurred())
			err = batch.Write()
			Expect(err).ToNot(HaveOccurred())

			batch.Reset()
			err = batch.Delete(testEthKey)
			Expect(err).ToNot(HaveOccurred())
			err = batch.Delete(testEthKey2)
			Expect(err).ToNot(HaveOccurred())
			err = batch.Write()
			Expect(err).ToNot(HaveOccurred())

			_, err = database.Get(testEthKey)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("block not found"))
			_, err = database.Get(testEthKey2)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("block not found"))
		})
	})

	Describe("ValueSize/Reset", func() {
		It("returns the size of data in the batch queued for write", func() {
			err = batch.Put(testEthKey, testValue)
			Expect(err).ToNot(HaveOccurred())
			err = batch.Put(testEthKey2, testValue2)
			Expect(err).ToNot(HaveOccurred())
			err = batch.Write()
			Expect(err).ToNot(HaveOccurred())

			size := batch.ValueSize()
			Expect(size).To(Equal(len(testValue) + len(testValue2)))

			batch.Reset()
			size = batch.ValueSize()
			Expect(size).To(Equal(0))
		})
	})
})
