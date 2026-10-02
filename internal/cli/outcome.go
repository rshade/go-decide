package cli

import (
	"fmt"

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
