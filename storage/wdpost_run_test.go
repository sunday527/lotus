//stm: #unit
package storage

import (
	"bytes"
	"context"
	"testing"

	prooftypes "github.com/filecoin-project/go-state-types/proof"

	"github.com/filecoin-project/lotus/chain/actors"
	lbuiltin "github.com/filecoin-project/lotus/chain/actors/builtin"
	"github.com/filecoin-project/lotus/chain/actors/builtin/miner"

	"github.com/stretchr/testify/require"
	"golang.org/x/xerrors"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/specs-storage/storage"

	"github.com/filecoin-project/go-state-types/builtin"
	minertypes "github.com/filecoin-project/go-state-types/builtin/v8/miner"
	tutils "github.com/filecoin-project/specs-actors/v2/support/testing"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/dline"
	"github.com/filecoin-project/go-state-types/network"

	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/actors/policy"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/extern/sector-storage/storiface"
	"github.com/filecoin-project/lotus/journal"
)

type mockStorageMinerAPI struct {
	partitions     []api.Partition
	pushedMessages chan *types.Message
	fullNodeFilteredAPI
}

func newMockStorageMinerAPI() *mockStorageMinerAPI {
	return &mockStorageMinerAPI{
		pushedMessages: make(chan *types.Message),
	}
}

func (m *mockStorageMinerAPI) StateMinerInfo(ctx context.Context, a address.Address, key types.TipSetKey) (api.MinerInfo, error) {
	return api.MinerInfo{
		Worker: tutils.NewIDAddr(nil, 101),
		Owner:  tutils.NewIDAddr(nil, 101),
	}, nil
}

func (m *mockStorageMinerAPI) StateNetworkVersion(ctx context.Context, key types.TipSetKey) (network.Version, error) {
	return build.NewestNetworkVersion, nil
}

func (m *mockStorageMinerAPI) StateGetRandomnessFromTickets(ctx context.Context, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte, tsk types.TipSetKey) (abi.Randomness, error) {
	return abi.Randomness("ticket rand"), nil
}

func (m *mockStorageMinerAPI) StateGetRandomnessFromBeacon(ctx context.Context, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte, tsk types.TipSetKey) (abi.Randomness, error) {
	return abi.Randomness("beacon rand"), nil
}

func (m *mockStorageMinerAPI) setPartitions(ps []api.Partition) {
	m.partitions = append(m.partitions, ps...)
}

func (m *mockStorageMinerAPI) StateMinerPartitions(ctx context.Context, a address.Address, dlIdx uint64, tsk types.TipSetKey) ([]api.Partition, error) {
	return m.partitions, nil
}

func (m *mockStorageMinerAPI) StateMinerSectors(ctx context.Context, address address.Address, snos *bitfield.BitField, key types.TipSetKey) ([]*minertypes.SectorOnChainInfo, error) {
	var sis []*minertypes.SectorOnChainInfo
	if snos == nil {
		panic("unsupported")
	}
	_ = snos.ForEach(func(i uint64) error {
		sis = append(sis, &minertypes.SectorOnChainInfo{
			SectorNumber: abi.SectorNumber(i),
		})
		return nil
	})
	return sis, nil
}

func (m *mockStorageMinerAPI) MpoolPushMessage(ctx context.Context, message *types.Message, spec *api.MessageSendSpec) (*types.SignedMessage, error) {
	m.pushedMessages <- message
	return &types.SignedMessage{
		Message: *message,
	}, nil
}

func (m *mockStorageMinerAPI) StateWaitMsg(ctx context.Context, cid cid.Cid, confidence uint64, limit abi.ChainEpoch, allowReplaced bool) (*api.MsgLookup, error) {
	return &api.MsgLookup{
		Receipt: types.MessageReceipt{
			ExitCode: 0,
		},
	}, nil
}

func (m *mockStorageMinerAPI) GasEstimateGasPremium(_ context.Context, nblocksincl uint64, sender address.Address, gaslimit int64, tsk types.TipSetKey) (types.BigInt, error) {
	return big.Zero(), nil
}

func (m *mockStorageMinerAPI) GasEstimateFeeCap(context.Context, *types.Message, int64, types.TipSetKey) (types.BigInt, error) {
	return big.Zero(), nil
}

type mockProver struct {
}

func (m *mockProver) GenerateWinningPoStWithVanilla(ctx context.Context, proofType abi.RegisteredPoStProof, minerID abi.ActorID, randomness abi.PoStRandomness, proofs [][]byte) ([]prooftypes.PoStProof, error) {
	panic("implement me")
}

func (m *mockProver) GenerateWindowPoStWithVanilla(ctx context.Context, proofType abi.RegisteredPoStProof, minerID abi.ActorID, randomness abi.PoStRandomness, proofs [][]byte, partitionIdx int) (prooftypes.PoStProof, error) {
	panic("implement me")
}

func (m *mockProver) GenerateWinningPoSt(context.Context, abi.ActorID, []prooftypes.ExtendedSectorInfo, abi.PoStRandomness) ([]prooftypes.PoStProof, error) {
	panic("implement me")
}

func (m *mockProver) GenerateWindowPoSt(ctx context.Context, aid abi.ActorID, sis []prooftypes.ExtendedSectorInfo, pr abi.PoStRandomness) ([]prooftypes.PoStProof, []abi.SectorID, error) {
	return []prooftypes.PoStProof{
		{
			PoStProof:  abi.RegisteredPoStProof_StackedDrgWindow2KiBV1,
			ProofBytes: []byte("post-proof"),
		},
	}, nil, nil
}

type mockVerif struct {
}

func (m mockVerif) VerifyWinningPoSt(ctx context.Context, info prooftypes.WinningPoStVerifyInfo) (bool, error) {
	panic("implement me")
}

func (m mockVerif) VerifyWindowPoSt(ctx context.Context, info prooftypes.WindowPoStVerifyInfo) (bool, error) {
	if len(info.Proofs) != 1 {
		return false, xerrors.Errorf("expected 1 proof entry")
	}

	proof := info.Proofs[0]

	if !bytes.Equal(proof.ProofBytes, []byte("post-proof")) {
		return false, xerrors.Errorf("bad proof")
	}
	return true, nil
}

func (m mockVerif) VerifyAggregateSeals(aggregate prooftypes.AggregateSealVerifyProofAndInfos) (bool, error) {
	panic("implement me")
}

func (m mockVerif) VerifyReplicaUpdate(update prooftypes.ReplicaUpdateInfo) (bool, error) {
	panic("implement me")
}

func (m mockVerif) VerifySeal(prooftypes.SealVerifyInfo) (bool, error) {
	panic("implement me")
}

func (m mockVerif) GenerateWinningPoStSectorChallenge(context.Context, abi.RegisteredPoStProof, abi.ActorID, abi.PoStRandomness, uint64) ([]uint64, error) {
	panic("implement me")
}

type mockFaultTracker struct {
}

func (m mockFaultTracker) CheckProvable(ctx context.Context, pp abi.RegisteredPoStProof, sectors []storage.SectorRef, rg storiface.RGetter) (map[abi.SectorID]string, error) {
	// Returns "bad" sectors so just return empty map meaning all sectors are good
	return map[abi.SectorID]string{}, nil
}

// TestWDPostDoPost verifies that doPost will send the correct number of window
// PoST messages for a given number of partitions
func TestWDPostDoPost(t *testing.T) {
	//stm: @CHAIN_SYNCER_LOAD_GENESIS_001, @CHAIN_SYNCER_FETCH_TIPSET_001,
	//stm: @CHAIN_SYNCER_START_001, @CHAIN_SYNCER_SYNC_001, @BLOCKCHAIN_BEACON_VALIDATE_BLOCK_VALUES_01
	//stm: @CHAIN_SYNCER_COLLECT_CHAIN_001, @CHAIN_SYNCER_COLLECT_HEADERS_001, @CHAIN_SYNCER_VALIDATE_TIPSET_001
	//stm: @CHAIN_SYNCER_NEW_PEER_HEAD_001, @CHAIN_SYNCER_VALIDATE_MESSAGE_META_001, @CHAIN_SYNCER_STOP_001
	ctx := context.Background()
	expectedMsgCount := 5

	proofType := abi.RegisteredPoStProof_StackedDrgWindow2KiBV1
	postAct := tutils.NewIDAddr(t, 100)

	mockStgMinerAPI := newMockStorageMinerAPI()

	// Get the number of sectors allowed in a partition for this proof type
	sectorsPerPartition, err := builtin.PoStProofWindowPoStPartitionSectors(proofType)
	require.NoError(t, err)
	// Work out the number of partitions that can be included in a message
	// without exceeding the message sector limit

	//stm: @BLOCKCHAIN_POLICY_GET_MAX_POST_PARTITIONS_001
	partitionsPerMsg, err := policy.GetMaxPoStPartitions(network.Version13, proofType)
	require.NoError(t, err)
	if partitionsPerMsg > minertypes.AddressedPartitionsMax {
		partitionsPerMsg = minertypes.AddressedPartitionsMax
	}

	// Enough partitions to fill expectedMsgCount-1 messages
	partitionCount := (expectedMsgCount - 1) * partitionsPerMsg
	// Add an extra partition that should be included in the last message
	partitionCount++

	var partitions []api.Partition
	for p := 0; p < partitionCount; p++ {
		sectors := bitfield.New()
		for s := uint64(0); s < sectorsPerPartition; s++ {
			sectors.Set(s)
		}
		partitions = append(partitions, api.Partition{
			AllSectors:        sectors,
			FaultySectors:     bitfield.New(),
			RecoveringSectors: bitfield.New(),
			LiveSectors:       sectors,
			ActiveSectors:     sectors,
		})
	}
	mockStgMinerAPI.setPartitions(partitions)

	// Run window PoST
	scheduler := &WindowPoStScheduler{
		api:          mockStgMinerAPI,
		prover:       &mockProver{},
		verifier:     &mockVerif{},
		faultTracker: &mockFaultTracker{},
		proofType:    proofType,
		actor:        postAct,
		journal:      journal.NilJournal(),
		addrSel:      &AddressSelector{},
	}

	di := &dline.Info{
		WPoStPeriodDeadlines:   minertypes.WPoStPeriodDeadlines,
		WPoStProvingPeriod:     minertypes.WPoStProvingPeriod,
		WPoStChallengeWindow:   minertypes.WPoStChallengeWindow,
		WPoStChallengeLookback: minertypes.WPoStChallengeLookback,
		FaultDeclarationCutoff: minertypes.FaultDeclarationCutoff,
	}
	ts := mockTipSet(t)

	scheduler.startGeneratePoST(ctx, ts, di, func(posts []minertypes.SubmitWindowedPoStParams, err error) {
		scheduler.startSubmitPoST(ctx, ts, di, posts, func(err error) {})
	})

	// Read the window PoST messages
	for i := 0; i < expectedMsgCount; i++ {
		msg := <-mockStgMinerAPI.pushedMessages
		require.Equal(t, builtin.MethodsMiner.SubmitWindowedPoSt, msg.Method)
		var params minertypes.SubmitWindowedPoStParams
		err := params.UnmarshalCBOR(bytes.NewReader(msg.Params))
		require.NoError(t, err)

		if i == expectedMsgCount-1 {
			// In the last message we only included a single partition (see above)
			require.Len(t, params.Partitions, 1)
		} else {
			// All previous messages should include the full number of partitions
			require.Len(t, params.Partitions, partitionsPerMsg)
		}
	}
}

// TestWDPostDoPostPartLimitConfig verifies that doPost will send the correct number of window
// PoST messages for a given number of partitions based on user config
func TestWDPostDoPostPartLimitConfig(t *testing.T) {
	//stm: @CHAIN_SYNCER_LOAD_GENESIS_001, @CHAIN_SYNCER_FETCH_TIPSET_001,
	//stm: @CHAIN_SYNCER_START_001, @CHAIN_SYNCER_SYNC_001, @BLOCKCHAIN_BEACON_VALIDATE_BLOCK_VALUES_01
	//stm: @CHAIN_SYNCER_COLLECT_CHAIN_001, @CHAIN_SYNCER_COLLECT_HEADERS_001, @CHAIN_SYNCER_VALIDATE_TIPSET_001
	//stm: @CHAIN_SYNCER_NEW_PEER_HEAD_001, @CHAIN_SYNCER_VALIDATE_MESSAGE_META_001, @CHAIN_SYNCER_STOP_001
	ctx := context.Background()
	expectedMsgCount := 364

	proofType := abi.RegisteredPoStProof_StackedDrgWindow2KiBV1
	postAct := tutils.NewIDAddr(t, 100)

	mockStgMinerAPI := newMockStorageMinerAPI()

	// Get the number of sectors allowed in a partition for this proof type
	sectorsPerPartition, err := builtin.PoStProofWindowPoStPartitionSectors(proofType)
	require.NoError(t, err)
	// Work out the number of partitions that can be included in a message
	// without exceeding the message sector limit

	//stm: @BLOCKCHAIN_POLICY_GET_MAX_POST_PARTITIONS_001
	partitionsPerMsg, err := policy.GetMaxPoStPartitions(network.Version13, proofType)
	require.NoError(t, err)
	if partitionsPerMsg > minertypes.AddressedPartitionsMax {
		partitionsPerMsg = minertypes.AddressedPartitionsMax
	}

	partitionCount := 4 * partitionsPerMsg

	// Assert that user config is less than network limit
	userPartLimit := 33
	lastMsgParts := 21
	require.Greater(t, partitionCount, userPartLimit)

	// Assert that we consts are correct
	require.Equal(t, (expectedMsgCount-1)*userPartLimit+lastMsgParts, 4*partitionsPerMsg)

	var partitions []api.Partition
	for p := 0; p < partitionCount; p++ {
		sectors := bitfield.New()
		for s := uint64(0); s < sectorsPerPartition; s++ {
			sectors.Set(s)
		}
		partitions = append(partitions, api.Partition{
			AllSectors:        sectors,
			FaultySectors:     bitfield.New(),
			RecoveringSectors: bitfield.New(),
			LiveSectors:       sectors,
			ActiveSectors:     sectors,
		})
	}
	mockStgMinerAPI.setPartitions(partitions)

	// Run window PoST
	scheduler := &WindowPoStScheduler{
		api:          mockStgMinerAPI,
		prover:       &mockProver{},
		verifier:     &mockVerif{},
		faultTracker: &mockFaultTracker{},
		proofType:    proofType,
		actor:        postAct,
		journal:      journal.NilJournal(),

		maxPartitionsPerPostMessage: userPartLimit,
	}

	di := &dline.Info{
		WPoStPeriodDeadlines:   minertypes.WPoStPeriodDeadlines,
		WPoStProvingPeriod:     minertypes.WPoStProvingPeriod,
		WPoStChallengeWindow:   minertypes.WPoStChallengeWindow,
		WPoStChallengeLookback: minertypes.WPoStChallengeLookback,
		FaultDeclarationCutoff: minertypes.FaultDeclarationCutoff,
	}
	ts := mockTipSet(t)

	scheduler.startGeneratePoST(ctx, ts, di, func(posts []minertypes.SubmitWindowedPoStParams, err error) {
		scheduler.startSubmitPoST(ctx, ts, di, posts, func(err error) {})
	})

	// Read the window PoST messages
	for i := 0; i < expectedMsgCount; i++ {
		msg := <-mockStgMinerAPI.pushedMessages
		require.Equal(t, builtin.MethodsMiner.SubmitWindowedPoSt, msg.Method)
		var params minertypes.SubmitWindowedPoStParams
		err := params.UnmarshalCBOR(bytes.NewReader(msg.Params))
		require.NoError(t, err)

		if i == expectedMsgCount-1 {
			// In the last message we only included a 21 partitions
			require.Len(t, params.Partitions, lastMsgParts)
		} else {
			// All previous messages should include the full number of partitions
			require.Len(t, params.Partitions, userPartLimit)
		}
	}
}

// TestWDPostDeclareRecoveriesPartLimitConfig verifies that declareRecoveries will send the correct number of
// DeclareFaultsRecovered messages for a given number of partitions based on user config
func TestWDPostDeclareRecoveriesPartLimitConfig(t *testing.T) {
	//stm: @CHAIN_SYNCER_LOAD_GENESIS_001, @CHAIN_SYNCER_FETCH_TIPSET_001,
	//stm: @CHAIN_SYNCER_START_001, @CHAIN_SYNCER_SYNC_001, @BLOCKCHAIN_BEACON_VALIDATE_BLOCK_VALUES_01
	//stm: @CHAIN_SYNCER_COLLECT_CHAIN_001, @CHAIN_SYNCER_COLLECT_HEADERS_001, @CHAIN_SYNCER_VALIDATE_TIPSET_001
	//stm: @CHAIN_SYNCER_NEW_PEER_HEAD_001, @CHAIN_SYNCER_VALIDATE_MESSAGE_META_001, @CHAIN_SYNCER_STOP_001
	ctx := context.Background()

	proofType := abi.RegisteredPoStProof_StackedDrgWindow2KiBV1
	postAct := tutils.NewIDAddr(t, 100)

	mockStgMinerAPI := newMockStorageMinerAPI()

	// Get the number of sectors allowed in a partition for this proof type
	sectorsPerPartition, err := builtin.PoStProofWindowPoStPartitionSectors(proofType)
	require.NoError(t, err)

	// Let's have 11/20 partitions with faulty sectors, and a config of 3 partitions per message
	userPartLimit := 3
	partitionCount := 20
	faultyPartitionCount := 11

	var partitions []api.Partition
	for p := 0; p < partitionCount; p++ {
		sectors := bitfield.New()
		for s := uint64(0); s < sectorsPerPartition; s++ {
			sectors.Set(s)
		}

		partition := api.Partition{
			AllSectors:        sectors,
			FaultySectors:     bitfield.New(),
			RecoveringSectors: bitfield.New(),
			LiveSectors:       sectors,
			ActiveSectors:     sectors,
		}

		if p < faultyPartitionCount {
			partition.FaultySectors = sectors
		}

		partitions = append(partitions, partition)
	}

	mockStgMinerAPI.setPartitions(partitions)

	// Run declareRecoverios
	scheduler := &WindowPoStScheduler{
		api:          mockStgMinerAPI,
		prover:       &mockProver{},
		verifier:     &mockVerif{},
		faultTracker: &mockFaultTracker{},
		proofType:    proofType,
		actor:        postAct,
		journal:      journal.NilJournal(),

		maxPartitionsPerRecoveryMessage: userPartLimit,
	}

	di := uint64(0)
	ts := mockTipSet(t)

	expectedMsgCount := faultyPartitionCount/userPartLimit + 1
	lastMsgParts := faultyPartitionCount % userPartLimit

	go func() {
		batchedRecoveries, msgs, err := scheduler.declareRecoveries(ctx, di, partitions, ts.Key())
		require.NoError(t, err, "failed to declare recoveries")
		require.Equal(t, len(batchedRecoveries), len(msgs))
		require.Equal(t, expectedMsgCount, len(msgs))
	}()

	// Read the window PoST messages
	for i := 0; i < expectedMsgCount; i++ {
		msg := <-mockStgMinerAPI.pushedMessages
		require.Equal(t, builtin.MethodsMiner.DeclareFaultsRecovered, msg.Method)
		var params minertypes.DeclareFaultsRecoveredParams
		err := params.UnmarshalCBOR(bytes.NewReader(msg.Params))
		require.NoError(t, err)

		if i == expectedMsgCount-1 {
			// In the last message we only included a 21 partitions
			require.Len(t, params.Recoveries, lastMsgParts)
		} else {
			// All previous messages should include the full number of partitions
			require.Len(t, params.Recoveries, userPartLimit)
		}

	}
}

func mockTipSet(t *testing.T) *types.TipSet {
	minerAct := tutils.NewActorAddr(t, "miner")
	c, err := cid.Decode("QmbFMke1KXqnYyBBWxB74N4c5SBnJMVAiMNRcGu6x1AwQH")
	require.NoError(t, err)
	blks := []*types.BlockHeader{
		{
			Miner:                 minerAct,
			Height:                abi.ChainEpoch(1),
			ParentStateRoot:       c,
			ParentMessageReceipts: c,
			Messages:              c,
		},
	}
	ts, err := types.NewTipSet(blks)
	require.NoError(t, err)
	return ts
}

//
// All the mock methods below here are unused
//

func (m *mockStorageMinerAPI) StateCall(ctx context.Context, message *types.Message, key types.TipSetKey) (*api.InvocResult, error) {
	panic("implement me")
}

func (m *mockStorageMinerAPI) StateMinerDeadlines(ctx context.Context, maddr address.Address, tok types.TipSetKey) ([]api.Deadline, error) {
	panic("implement me")
}

func (m *mockStorageMinerAPI) StateSectorPreCommitInfo(ctx context.Context, address address.Address, number abi.SectorNumber, key types.TipSetKey) (minertypes.SectorPreCommitOnChainInfo, error) {
	panic("implement me")
}

func (m *mockStorageMinerAPI) StateSectorGetInfo(ctx context.Context, address address.Address, number abi.SectorNumber, key types.TipSetKey) (*minertypes.SectorOnChainInfo, error) {
	panic("implement me")
}

func (m *mockStorageMinerAPI) StateSectorPartition(ctx context.Context, maddr address.Address, sectorNumber abi.SectorNumber, tok types.TipSetKey) (*miner.SectorLocation, error) {
	panic("implement me")
}

func (m *mockStorageMinerAPI) StateMinerProvingDeadline(ctx context.Context, address address.Address, key types.TipSetKey) (*dline.Info, error) {
	return &dline.Info{
		CurrentEpoch:           0,
		PeriodStart:            0,
		Index:                  0,
		Open:                   0,
		Close:                  0,
		Challenge:              0,
		FaultCutoff:            0,
		WPoStPeriodDeadlines:   minertypes.WPoStPeriodDeadlines,
		WPoStProvingPeriod:     minertypes.WPoStProvingPeriod,
		WPoStChallengeWindow:   minertypes.WPoStChallengeWindow,
		WPoStChallengeLookback: minertypes.WPoStChallengeLookback,
		FaultDeclarationCutoff: minertypes.FaultDeclarationCutoff,
	}, nil
}

func (m *mockStorageMinerAPI) StateMinerPreCommitDepositForPower(ctx context.Context, address address.Address, info minertypes.SectorPreCommitInfo, key types.TipSetKey) (types.BigInt, error) {
	panic("implement me")
}

func (m *mockStorageMinerAPI) StateMinerInitialPledgeCollateral(ctx context.Context, address address.Address, info minertypes.SectorPreCommitInfo, key types.TipSetKey) (types.BigInt, error) {
	panic("implement me")
}

func (m *mockStorageMinerAPI) StateSearchMsg(ctx context.Context, from types.TipSetKey, msg cid.Cid, limit abi.ChainEpoch, allowReplaced bool) (*api.MsgLookup, error) {
	panic("implement me")
}

func (m *mockStorageMinerAPI) StateGetActor(ctx context.Context, actor address.Address, ts types.TipSetKey) (*types.Actor, error) {
	code, err := lbuiltin.GetMinerActorCodeID(actors.Version7)
	if err != nil {
		return nil, err
	}
	return &types.Actor{
		Code: code,
	}, nil
}

func (m *mockStorageMinerAPI) StateGetReceipt(ctx context.Context, cid cid.Cid, key types.TipSetKey) (*types.MessageReceipt, error) {
	panic("implement me")
}

func (m *mockStorageMinerAPI) StateMarketStorageDeal(ctx context.Context, id abi.DealID, key types.TipSetKey) (*api.MarketDeal, error) {
	panic("implement me")
}

func (m *mockStorageMinerAPI) StateMinerFaults(ctx context.Context, address address.Address, key types.TipSetKey) (bitfield.BitField, error) {
	panic("implement me")
}

func (m *mockStorageMinerAPI) StateMinerRecoveries(ctx context.Context, address address.Address, key types.TipSetKey) (bitfield.BitField, error) {
	panic("implement me")
}

func (m *mockStorageMinerAPI) StateAccountKey(ctx context.Context, address address.Address, key types.TipSetKey) (address.Address, error) {
	return address, nil
}

func (m *mockStorageMinerAPI) GasEstimateMessageGas(ctx context.Context, message *types.Message, spec *api.MessageSendSpec, key types.TipSetKey) (*types.Message, error) {
	msg := *message
	msg.GasFeeCap = big.NewInt(1)
	msg.GasPremium = big.NewInt(1)
	msg.GasLimit = 2
	return &msg, nil
}

func (m *mockStorageMinerAPI) ChainHead(ctx context.Context) (*types.TipSet, error) {
	return nil, nil
}

func (m *mockStorageMinerAPI) ChainNotify(ctx context.Context) (<-chan []*api.HeadChange, error) {
	panic("implement me")
}

func (m *mockStorageMinerAPI) ChainGetTipSetByHeight(ctx context.Context, epoch abi.ChainEpoch, key types.TipSetKey) (*types.TipSet, error) {
	panic("implement me")
}

func (m *mockStorageMinerAPI) ChainGetBlockMessages(ctx context.Context, cid cid.Cid) (*api.BlockMessages, error) {
	panic("implement me")
}

func (m *mockStorageMinerAPI) ChainReadObj(ctx context.Context, cid cid.Cid) ([]byte, error) {
	panic("implement me")
}

func (m *mockStorageMinerAPI) ChainHasObj(ctx context.Context, cid cid.Cid) (bool, error) {
	panic("implement me")
}

func (m *mockStorageMinerAPI) ChainGetTipSet(ctx context.Context, key types.TipSetKey) (*types.TipSet, error) {
	panic("implement me")
}

func (m *mockStorageMinerAPI) WalletSign(ctx context.Context, address address.Address, bytes []byte) (*crypto.Signature, error) {
	return nil, nil
}

func (m *mockStorageMinerAPI) WalletBalance(ctx context.Context, address address.Address) (types.BigInt, error) {
	return big.NewInt(333), nil
}

func (m *mockStorageMinerAPI) WalletHas(ctx context.Context, address address.Address) (bool, error) {
	return true, nil
}

var _ fullNodeFilteredAPI = &mockStorageMinerAPI{}
