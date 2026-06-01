package keeper

import (
	"strings"

	"github.com/cosmos/cosmos-sdk/runtime"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	host "github.com/cosmos/ibc-go/v11/modules/core/24-host"
	"github.com/cosmos/ibc-go/v11/modules/core/exported"
)

// Migrator is a struct for handling in-place store migrations.
type Migrator struct {
	keeper *Keeper
}

// NewMigrator returns a new Migrator.
func NewMigrator(keeper *Keeper) Migrator {
	return Migrator{keeper: keeper}
}

// MigrateToStatelessLocalhost deletes the localhost client state. The localhost
// implementation is now stateless.
func (m Migrator) MigrateToStatelessLocalhost(ctx sdk.Context) error {
	clientStore := m.keeper.ClientStore(ctx, exported.LocalhostClientID)

	// delete the client state
	clientStore.Delete(host.ClientStateKey())
	return nil
}

// PruneStaleConsensusStateSubkeys removes stale keys of the form
// clients/<id>/consensusStates/<revision>/<height>/clientState that were left
// behind when old-format consensus state entries were migrated to the flat
// consensusStates/<revision>-<height> layout. The v7 migration only cleaned
// canonical 2-segment keys; 3-segment keys survived and can cause the
// ClientStates query to attempt to unmarshal ConsensusState bytes as
// ClientState, triggering a proto-decoder panic.
func (m Migrator) PruneStaleConsensusStateSubkeys(ctx sdk.Context) error {
	store := runtime.KVStoreAdapter(m.keeper.storeService.OpenKVStore(ctx))
	iterator := storetypes.KVStorePrefixIterator(store, host.KeyClientStorePrefix)
	defer sdk.LogDeferred(ctx.Logger(), func() error { return iterator.Close() })

	var staleKeys [][]byte
	for ; iterator.Valid(); iterator.Next() {
		// iterator.Key() returns the full key including the "clients/" prefix.
		// Canonical clientState key:  clients/<id>/clientState           (3 parts)
		// Canonical consensusState:   clients/<id>/consensusStates/<h>   (4 parts)
		// Stale artifacts (caught by len >= 5):
		//   clients/<id>/consensusStates/<h>/clientState          (5 parts, rev-less old format)
		//   clients/<id>/consensusStates/<rev>/<h>/clientState    (6 parts, observed on Cronos)
		parts := strings.Split(string(iterator.Key()), "/")
		if len(parts) >= 5 &&
			parts[2] == host.KeyConsensusStatePrefix &&
			parts[len(parts)-1] == host.KeyClientState {
			k := make([]byte, len(iterator.Key()))
			copy(k, iterator.Key())
			staleKeys = append(staleKeys, k)
		}
	}

	for _, k := range staleKeys {
		store.Delete(k)
	}
	return nil
}
