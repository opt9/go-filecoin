package vmcontext

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
)

// GasTracker maintains the state of gas usage throughout the execution of a message.
type GasTracker struct {
	gasLimit    gas.Unit
	gasConsumed gas.Unit
}

// NewGasTracker initializes a new empty gas tracker
func NewGasTracker(limit gas.Unit) GasTracker {
	return GasTracker{
		gasLimit:    limit,
		gasConsumed: gas.Zero,
	}
}

// Charge will add the gas charge to the current method gas context.
//
// WARNING: this method will panic if there is no sufficient gas left.
func (t *GasTracker) Charge(amount gas.Unit) {
	if ok := t.TryCharge(amount); !ok {
		runtime.Abort(exitcode.SysErrOutOfGas)
	}
}

// TryCharge charges `amount` or `RemainingGas()``, whichever is smaller.
//
// Returns `True` if the there was enough gas to pay for `amount`.
func (t *GasTracker) TryCharge(amount gas.Unit) bool {
	// check for limit
	aux := big.Add(t.gasConsumed.AsBigInt(), amount.AsBigInt())
	if aux.GreaterThan(t.gasLimit.AsBigInt()) {
		t.gasConsumed = t.gasLimit
		return false
	}

	t.gasConsumed = gas.Unit(aux)
	return true
}

// GasConsumed returns the gas consumed.
func (t *GasTracker) GasConsumed() gas.Unit {
	return t.gasConsumed
}

// RemainingGas returns the gas remaining.
func (t *GasTracker) RemainingGas() gas.Unit {
	return gas.Unit(big.Sub(t.gasLimit.AsBigInt(), t.gasConsumed.AsBigInt()))
}
