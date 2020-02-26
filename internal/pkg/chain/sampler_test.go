package chain_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"strconv"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/minio/blake2b-simd"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestSamplingChainRandomness(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()

	makeSample := func(targetEpoch, sampleEpoch int) crypto.RandomSeed {
		buf := bytes.Buffer{}
		var vrfProof []byte
		if sampleEpoch >= 0 {
			vrfProof = []byte(strconv.Itoa(sampleEpoch))
		}
		vrfDigest := blake2b.Sum256(vrfProof)
		buf.Write(vrfDigest[:])
		_ = binary.Write(&buf, binary.BigEndian, abi.ChainEpoch(targetEpoch))
		bufHash := blake2b.Sum256(buf.Bytes())
		return bufHash[:]
	}

	t.Run("happy path", func(t *testing.T) {
		builder, ch := makeChain(t, 21)
		head := ch[0].Key()
		sampler := chain.NewSampler(builder)

		r, err := sampler.Sample(ctx, head, abi.ChainEpoch(20))
		assert.NoError(t, err)
		assert.Equal(t, makeSample(20, 20), r)

		r, err = sampler.Sample(ctx, head, abi.ChainEpoch(3))
		assert.NoError(t, err)
		assert.Equal(t, makeSample(3, 3), r)

		r, err = sampler.Sample(ctx, head, abi.ChainEpoch(0))
		assert.NoError(t, err)
		assert.Equal(t, makeSample(0, 0), r)
	})

	t.Run("skips missing tipsets", func(t *testing.T) {
		builder, ch := makeChain(t, 21)
		head := ch[0].Key()
		sampler := chain.NewSampler(builder)

		// Sample height after the head falls back to the head.
		headParent := ch[1].Key()
		r, err := sampler.Sample(ctx, headParent, abi.ChainEpoch(20))
		assert.NoError(t, err)
		assert.Equal(t, makeSample(20, 19), r)

		// Another way of the same thing, sample > head.
		r, err = sampler.Sample(ctx, head, abi.ChainEpoch(21))
		assert.NoError(t, err)
		assert.Equal(t, makeSample(21, 20), r)

		// Add new head so as to produce null blocks between 20 and 25
		// i.e.: 25 20 19 18 ... 0
		headAfterNulls := builder.BuildOneOn(ch[0], func(b *chain.BlockBuilder) {
			b.IncHeight(4)
			b.SetTicket([]byte(strconv.Itoa(25)))
		})

		// Sampling in the nulls falls back to the last non-null
		r, err = sampler.Sample(ctx, headAfterNulls.Key(), abi.ChainEpoch(24))
		assert.NoError(t, err)
		assert.Equal(t, makeSample(24, 20), r)
	})

	t.Run("genesis", func(t *testing.T) {
		builder, ch := makeChain(t, 6)
		head := ch[0].Key()
		gen := (ch[len(ch)-1]).Key()
		sampler := chain.NewSampler(builder)

		// Sample genesis from longer chain.
		r, err := sampler.Sample(ctx, head, abi.ChainEpoch(0))
		assert.NoError(t, err)
		assert.Equal(t, makeSample(0, 0), r)

		// Sample before genesis from longer chain.
		r, err = sampler.Sample(ctx, head, abi.ChainEpoch(-1))
		assert.NoError(t, err)
		assert.Equal(t, makeSample(-1, 0), r)

		// Sample genesis from genesis-only chain.
		r, err = sampler.Sample(ctx, gen, abi.ChainEpoch(0))
		assert.NoError(t, err)
		assert.Equal(t, makeSample(0, 0), r)

		// Sample before genesis from genesis-only chain.
		r, err = sampler.Sample(ctx, gen, abi.ChainEpoch(-1))
		assert.NoError(t, err)
		assert.Equal(t, makeSample(-1, 0), r)

		// Sample empty chain.
		r, err = sampler.Sample(ctx, block.NewTipSetKey(), abi.ChainEpoch(0))
		assert.NoError(t, err)
		assert.Equal(t, makeSample(0, -1), r)
		r, err = sampler.Sample(ctx, block.NewTipSetKey(), abi.ChainEpoch(-1))
		assert.NoError(t, err)
		assert.Equal(t, makeSample(-1, -1), r)
	})
}

// Builds a chain of single-block tips, returned in descending height order.
// Each block's ticket is its stringified height (as bytes).
func makeChain(t *testing.T, length int) (*chain.Builder, []block.TipSet) {
	b := chain.NewBuilder(t, address.Undef)
	height := 0
	head := b.BuildManyOn(length, block.UndefTipSet, func(b *chain.BlockBuilder) {
		b.SetTicket([]byte(strconv.Itoa(height)))
		height++
	})
	return b, b.RequireTipSets(head.Key(), length)
}
