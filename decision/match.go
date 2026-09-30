package decision

// Match runs the handler for r's case and returns its value. It needs a handler
// for each of the three cases, so leaving one out does not compile.
//
// A nil Result, which is the only way to hold a result that was never produced,
// runs onEscalate with an empty [Escalate] value: no leading option and no
// valid confidence. It never runs onDecided.
func Match[T ~string, R any](
	r Result[T],
	onDecided func(Decided[T]) R,
	onUncertain func(Uncertain[T]) R,
	onEscalate func(Escalate[T]) R,
) R {
	switch v := r.(type) {
	case Decided[T]:
		return onDecided(v)
	case Uncertain[T]:
		return onUncertain(v)
	case Escalate[T]:
		return onEscalate(v)
	default:
		return onEscalate(escalateResult[T]{})
	}
}
