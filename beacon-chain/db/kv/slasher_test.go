package kv

import (
	"context"
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	slashertypes "github.com/prysmaticlabs/prysm/beacon-chain/slasher/types"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestStore_AttestationRecordForValidator_SaveRetrieve(t *testing.T) {
	ctx := context.Background()
	beaconDB := setupDB(t)
	valIdx := types.ValidatorIndex(1)
	target := types.Epoch(5)
	source := types.Epoch(4)
	attRecord, err := beaconDB.AttestationRecordForValidator(ctx, valIdx, target)
	require.NoError(t, err)
	require.Equal(t, true, attRecord == nil)

	sr := [32]byte{1}
	err = beaconDB.SaveAttestationRecordsForValidators(ctx, []*slashertypes.CompactAttestation{
		{
			AttestingIndices: []uint64{uint64(valIdx)},
			Target:           target,
			Source:           source,
			SigningRoot:      sr,
		},
	})
	require.NoError(t, err)
	attRecord, err = beaconDB.AttestationRecordForValidator(ctx, valIdx, target)
	require.NoError(t, err)
	assert.DeepEqual(t, target, attRecord.Target)
	assert.DeepEqual(t, source, attRecord.Source)
	assert.DeepEqual(t, sr, attRecord.SigningRoot)
}

func TestStore_LatestEpochAttestedForValidators(t *testing.T) {
	ctx := context.Background()
	beaconDB := setupDB(t)
	indices := []types.ValidatorIndex{1, 2, 3}
	epoch := types.Epoch(5)

	attestedEpochs, err := beaconDB.LatestEpochAttestedForValidators(ctx, indices)
	require.NoError(t, err)
	require.Equal(t, true, len(attestedEpochs) == 0)

	err = beaconDB.SaveLatestEpochAttestedForValidators(ctx, indices, epoch)
	require.NoError(t, err)

	retrievedEpochs, err := beaconDB.LatestEpochAttestedForValidators(ctx, indices)
	require.NoError(t, err)
	require.Equal(t, len(indices), len(retrievedEpochs))

	for i, retrievedEpoch := range retrievedEpochs {
		want := &slashertypes.AttestedEpochForValidator{
			Epoch:          epoch,
			ValidatorIndex: indices[i],
		}
		require.DeepEqual(t, want, retrievedEpoch)
	}
}

func TestStore_CheckAttesterDoubleVotes(t *testing.T) {
	ctx := context.Background()
	beaconDB := setupDB(t)
	err := beaconDB.SaveAttestationRecordsForValidators(ctx, []*slashertypes.CompactAttestation{
		{
			AttestingIndices: []uint64{0, 1},
			Source:           2,
			Target:           3,
			SigningRoot:      [32]byte{1},
		},
		{
			AttestingIndices: []uint64{2, 3},
			Source:           3,
			Target:           4,
			SigningRoot:      [32]byte{1},
		},
	})
	require.NoError(t, err)

	slashableAtts := []*slashertypes.CompactAttestation{
		{
			AttestingIndices: []uint64{0, 1},
			Source:           2,
			Target:           3,
			SigningRoot:      [32]byte{2}, // Different signing root.
		},
		{
			AttestingIndices: []uint64{2, 3},
			Source:           3,
			Target:           4,
			SigningRoot:      [32]byte{2}, // Different signing root.
		},
	}

	wanted := []*slashertypes.AttesterDoubleVote{
		{
			ValidatorIndex:  0,
			SigningRoot:     [32]byte{2},
			PrevSigningRoot: [32]byte{1},
			Target:          3,
		},
		{
			ValidatorIndex:  1,
			SigningRoot:     [32]byte{2},
			PrevSigningRoot: [32]byte{1},
			Target:          3,
		},
		{
			ValidatorIndex:  2,
			SigningRoot:     [32]byte{2},
			PrevSigningRoot: [32]byte{1},
			Target:          4,
		},
		{
			ValidatorIndex:  3,
			SigningRoot:     [32]byte{2},
			PrevSigningRoot: [32]byte{1},
			Target:          4,
		},
	}
	doubleVotes, err := beaconDB.CheckAttesterDoubleVotes(ctx, slashableAtts)
	require.NoError(t, err)
	require.DeepEqual(t, wanted, doubleVotes)
}

func TestStore_SlasherChunk_SaveRetrieve(t *testing.T) {
	ctx := context.Background()
	beaconDB := setupDB(t)
	elemsPerChunk := 16
	totalChunks := 64
	chunkKeys := make([]uint64, totalChunks)
	chunks := make([][]uint16, totalChunks)
	for i := 0; i < totalChunks; i++ {
		chunk := make([]uint16, elemsPerChunk)
		for j := 0; j < len(chunk); j++ {
			chunk[j] = uint16(0)
		}
		chunks[i] = chunk
		chunkKeys[i] = uint64(i)
	}

	// We save chunks for min spans.
	err := beaconDB.SaveSlasherChunks(ctx, slashertypes.MinSpan, chunkKeys, chunks)
	require.NoError(t, err)

	// We expect no chunks to be stored for max spans.
	_, chunksExist, err := beaconDB.LoadSlasherChunks(
		ctx, slashertypes.MaxSpan, chunkKeys,
	)
	require.NoError(t, err)
	require.Equal(t, len(chunks), len(chunksExist))
	for _, exists := range chunksExist {
		require.Equal(t, false, exists)
	}

	// We check we saved the right chunks.
	retrievedChunks, chunksExist, err := beaconDB.LoadSlasherChunks(
		ctx, slashertypes.MinSpan, chunkKeys,
	)
	require.NoError(t, err)
	require.Equal(t, len(chunks), len(retrievedChunks))
	require.Equal(t, len(chunks), len(chunksExist))
	for i, exists := range chunksExist {
		require.Equal(t, true, exists)
		require.DeepEqual(t, chunks[i], retrievedChunks[i])
	}

	// We save chunks for max spans.
	err = beaconDB.SaveSlasherChunks(ctx, slashertypes.MaxSpan, chunkKeys, chunks)
	require.NoError(t, err)

	// We check we saved the right chunks.
	retrievedChunks, chunksExist, err = beaconDB.LoadSlasherChunks(
		ctx, slashertypes.MaxSpan, chunkKeys,
	)
	require.NoError(t, err)
	require.Equal(t, len(chunks), len(retrievedChunks))
	require.Equal(t, len(chunks), len(chunksExist))
	for i, exists := range chunksExist {
		require.Equal(t, true, exists)
		require.DeepEqual(t, chunks[i], retrievedChunks[i])
	}
}