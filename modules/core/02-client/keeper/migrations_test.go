package keeper_test

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/runtime"

	"github.com/cosmos/ibc-go/v11/modules/core/02-client/keeper"
	host "github.com/cosmos/ibc-go/v11/modules/core/24-host"
	ibcexported "github.com/cosmos/ibc-go/v11/modules/core/exported"
)

func (s *KeeperTestSuite) TestMigrateToStatelessLocalhost() {
	// set localhost in state
	clientStore := s.chainA.GetSimApp().IBCKeeper.ClientKeeper.ClientStore(s.chainA.GetContext(), ibcexported.LocalhostClientID)
	clientStore.Set(host.ClientStateKey(), []byte("clientState"))

	m := keeper.NewMigrator(s.chainA.GetSimApp().IBCKeeper.ClientKeeper)
	err := m.MigrateToStatelessLocalhost(s.chainA.GetContext())
	s.Require().NoError(err)
	s.Require().False(clientStore.Has(host.ClientStateKey()))

	// rerun migration on no localhost set
	err = m.MigrateToStatelessLocalhost(s.chainA.GetContext())
	s.Require().NoError(err)
	s.Require().False(clientStore.Has(host.ClientStateKey()))
}

// TestPruneStaleConsensusStateSubkeys verifies that stale
// clients/<id>/consensusStates/<rev>/<height>/clientState keys are deleted while
// canonical clientState and consensusState keys are preserved.
func (s *KeeperTestSuite) TestPruneStaleConsensusStateSubkeys() {
	ctx := s.chainA.GetContext()
	ibcStore := runtime.KVStoreAdapter(
		runtime.NewKVStoreService(s.chainA.GetSimApp().GetKey(ibcexported.StoreKey)).OpenKVStore(ctx),
	)

	const (
		clientA = "07-tendermint-0"
		clientB = "07-tendermint-1"
	)

	// canonical keys — must not be deleted
	canonical := []string{
		fmt.Sprintf("clients/%s/%s", clientA, host.KeyClientState),
		fmt.Sprintf("clients/%s/%s", clientB, host.KeyClientState),
		fmt.Sprintf("clients/%s/%s/0-100", clientA, host.KeyConsensusStatePrefix),
		fmt.Sprintf("clients/%s/%s/1-200", clientB, host.KeyConsensusStatePrefix),
	}
	for _, k := range canonical {
		ibcStore.Set([]byte(k), []byte("canonical-value"))
	}

	// stale old-format keys — must be deleted
	stale := []string{
		fmt.Sprintf("clients/%s/%s/0/50/%s", clientB, host.KeyConsensusStatePrefix, host.KeyClientState),
		fmt.Sprintf("clients/%s/%s/1/200/%s", clientB, host.KeyConsensusStatePrefix, host.KeyClientState),
		fmt.Sprintf("clients/%s/%s/0/100/%s", clientA, host.KeyConsensusStatePrefix, host.KeyClientState),
	}
	for _, k := range stale {
		ibcStore.Set([]byte(k), []byte("consensus-bytes-stored-under-clientState-key"))
	}

	m := keeper.NewMigrator(s.chainA.GetSimApp().IBCKeeper.ClientKeeper)
	s.Require().NoError(m.PruneStaleConsensusStateSubkeys(ctx))

	for _, k := range stale {
		s.Require().Nil(ibcStore.Get([]byte(k)), "stale key %s should be deleted", k)
	}
	for _, k := range canonical {
		s.Require().NotNil(ibcStore.Get([]byte(k)), "canonical key %s must not be deleted", k)
	}

	// idempotent: second run must not error and must not touch canonical keys
	s.Require().NoError(m.PruneStaleConsensusStateSubkeys(ctx))
	for _, k := range canonical {
		s.Require().NotNil(ibcStore.Get([]byte(k)), "canonical key %s must survive second migration run", k)
	}
}

