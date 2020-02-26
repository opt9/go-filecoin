package chain

import (
	"bytes"
	"context"
	"encoding/binary"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/minio/blake2b-simd"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
)

// Creates a new sampler for the chain identified by `head`.
func NewSamplerAtHead(reader TipSetProvider, head block.TipSetKey) *SamplerAtHead {
	return &SamplerAtHead{
		sampler: NewSampler(reader),
		head:    head,
	}
}

// A sampler draws randomness seeds from the chain. The seed is computed from the minimum ticket of the tipset
// at or before the requested epoch, mixed with the epoch itself (and is thus unique per epoch, even when they are
// empty).
//
// This implementation doesn't do any caching: it traverses the chain each time. A cache that could be directly
// indexed by epoch could speed up repeated samples from the same chain.
type Sampler struct {
	reader TipSetProvider
}

func NewSampler(reader TipSetProvider) *Sampler {
	return &Sampler{reader}
}

// Draws a randomness seed from the chain identified by `head` and the highest tipset with height <= `epoch`.
// If `head` is empty (as when processing the genesis block), the seed is empty.
func (s *Sampler) Sample(ctx context.Context, head block.TipSetKey, epoch abi.ChainEpoch) (crypto.RandomSeed, error) {
	var ticket block.Ticket
	if !head.Empty() {
		start, err := s.reader.GetTipSet(head)
		if err != nil {
			return nil, err
		}
		// Note: it is not an error to have epoch > start.Height(); in the case of a run of null blocks the
		// sought-after height may be after the base (last non-empty) tipset.
		// It's also not an error for the requested epoch to be negative.

		tip, err := s.findTipsetAtEpoch(ctx, start, epoch)
		if err != nil {
			return nil, err
		}
		ticket, err = tip.MinTicket()
		if err != nil {
			return nil, err
		}
	} else {
		// Sampling for the genesis block.
		ticket.VRFProof = []byte{}
	}

	buf := bytes.Buffer{}
	vrfDigest := blake2b.Sum256(ticket.VRFProof)
	buf.Write(vrfDigest[:])
	err := binary.Write(&buf, binary.BigEndian, epoch)
	if err != nil {
		return nil, err
	}

	bufHash := blake2b.Sum256(buf.Bytes())
	return bufHash[:], err
}

// Finds the the highest tipset with height <= the requested epoch, by traversing backward from start.
func (s *Sampler) findTipsetAtEpoch(ctx context.Context, start block.TipSet, epoch abi.ChainEpoch) (ts block.TipSet, err error) {
	iterator := IterAncestors(ctx, s.reader, start)
	var h abi.ChainEpoch
	for ; !iterator.Complete(); err = iterator.Next() {
		if err != nil {
			return
		}
		ts = iterator.Value()
		h, err = ts.Height()
		if err != nil {
			return
		}
		if h <= epoch {
			break
		}
	}
	// If the iterator completed, ts is the genesis tipset.
	return
}

///// A chain sampler with a specific head tipset key. /////

type SamplerAtHead struct {
	sampler *Sampler
	head    block.TipSetKey
}

func (s *SamplerAtHead) Sample(ctx context.Context, epoch abi.ChainEpoch) (crypto.RandomSeed, error) {
	return s.sampler.Sample(ctx, s.head, epoch)
}
