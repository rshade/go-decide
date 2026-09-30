package decision

// kind is which of the three results a confidence produces. The zero value is
// escalate, so a kind that was never set never reads as a decision.
type kind int

const (
	kindEscalate kind = iota
	kindUncertain
	kindDecided
)

func (k kind) String() string {
	switch k {
	case kindDecided:
		return "decided"
	case kindUncertain:
		return "uncertain"
	default:
		return "escalate"
	}
}

// classify maps a confidence to a result kind. A confidence or thresholds that
// are not valid escalate, the safe direction.
func classify(confidence Probability, th Thresholds) kind {
	switch {
	case !confidence.Valid() || !th.Valid():
		return kindEscalate
	case confidence.AtLeast(th.Confident()):
		return kindDecided
	case confidence.AtLeast(th.Floor()):
		return kindUncertain
	default:
		return kindEscalate
	}
}
