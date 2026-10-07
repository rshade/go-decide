package cli

import (
	"fmt"
	"sync/atomic"

	ax "github.com/rshade/ax-go"
)

// outcomeExitCodes is the exit-code line of the ask and score help text.
var outcomeExitCodes = fmt.Sprintf("Exit codes: 0 decided, %d uncertain, %d escalate", ExitUncertain, ExitEscalate)

type outcomeKind int

const (
	outcomeNone outcomeKind = iota
	outcomeDecided
	outcomeUncertain
	outcomeEscalate
)

// outcomeRecorder holds the outcome the last ask or score call produced. Calls
// can overlap when go-decide serves MCP tools, so it is safe for concurrent use.
type outcomeRecorder struct {
	kind atomic.Int32
}

func (r *outcomeRecorder) record(kind outcomeKind) { r.kind.Store(int32(kind)) }

func (r *outcomeRecorder) recorded() outcomeKind { return outcomeKind(r.kind.Load()) }

func (o outcomeKind) name() string {
	switch o {
	case outcomeDecided:
		return "decided"
	case outcomeUncertain:
		return "uncertain"
	default:
		return "escalate"
	}
}

func (o outcomeKind) exitCode() int {
	switch o {
	case outcomeUncertain:
		return ExitUncertain
	case outcomeEscalate:
		return ExitEscalate
	default:
		return ax.ExitSuccess
	}
}
