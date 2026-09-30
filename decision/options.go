package decision

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

// ErrInvalidOptions is wrapped by the error [NewOptions] returns for a set
// that cannot be asked as a choice.
var ErrInvalidOptions = errors.New("decision: invalid options")

// Options is the set of choices a question offers, each with a description of
// when it applies. Create one with [NewOptions]; the zero value is not valid.
type Options[T ~string] struct {
	descriptions map[T]string
}

// NewOptions validates and copies descriptions, which maps each option to when
// it applies. A description may be empty, and Jev then reads the name alone. It
// fails for fewer than two options or an empty option name, so a bad set is
// rejected before any request is sent.
func NewOptions[T ~string](descriptions map[T]string) (Options[T], error) {
	if len(descriptions) < 2 {
		return Options[T]{}, fmt.Errorf("%w: a choice needs at least two options, got %d",
			ErrInvalidOptions, len(descriptions))
	}
	if _, empty := descriptions[""]; empty {
		return Options[T]{}, fmt.Errorf("%w: an option name is empty", ErrInvalidOptions)
	}
	return Options[T]{descriptions: maps.Clone(descriptions)}, nil
}

// Valid reports whether o was created by [NewOptions].
func (o Options[T]) Valid() bool { return len(o.descriptions) >= 2 }

// Len returns the number of options.
func (o Options[T]) Len() int { return len(o.descriptions) }

// Contains reports whether name is one of the options.
func (o Options[T]) Contains(name T) bool {
	_, ok := o.descriptions[name]
	return ok
}

// Description returns when name applies and whether name is an option.
func (o Options[T]) Description(name T) (string, bool) {
	d, ok := o.descriptions[name]
	return d, ok
}

// Names returns the options in sorted order.
func (o Options[T]) Names() []T {
	return slices.Sorted(maps.Keys(o.descriptions))
}
