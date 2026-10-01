package decision

import (
	"errors"
	"slices"
)

// ErrInvalidLevels is wrapped by the error [NewLevels] returns for a rubric
// that cannot be asked as a score question.
var ErrInvalidLevels = errors.New("decision: invalid levels")

// MaxLevels is the most levels a score question can have.
const MaxLevels = 10

// Level is one step of a rubric: a name and when it applies.
type Level[T ~string] struct {
	Name T
	// Description says when the level applies. Jev reads the name alone when it
	// is empty.
	Description string
}

// Levels is the ordered rubric of a score question, lowest level first. Create
// one with [NewLevels]; the zero value is not valid.
type Levels[T ~string] struct {
	levels []Level[T]
}

// NewLevels validates and copies levels, which are ordered lowest first. It
// fails for fewer than two levels, more than [MaxLevels], an empty name or a
// repeated name, with a [*FieldError] on Levels.
func NewLevels[T ~string](levels ...Level[T]) (Levels[T], error) {
	if len(levels) < 2 {
		return Levels[T]{}, newFieldError(ErrInvalidLevels, "Levels",
			"a rubric needs at least two levels, got %d", len(levels))
	}
	if len(levels) > MaxLevels {
		return Levels[T]{}, newFieldError(ErrInvalidLevels, "Levels",
			"a rubric takes at most %d levels, got %d", MaxLevels, len(levels))
	}
	seen := make(map[T]int, len(levels))
	for i, level := range levels {
		if level.Name == "" {
			return Levels[T]{}, newFieldError(ErrInvalidLevels, "Levels", "level %d has an empty name", i+1)
		}
		if first, dup := seen[level.Name]; dup {
			return Levels[T]{}, newFieldError(ErrInvalidLevels, "Levels",
				"name %q is used by levels %d and %d", level.Name, first+1, i+1)
		}
		seen[level.Name] = i
	}
	return Levels[T]{levels: slices.Clone(levels)}, nil
}

// Valid reports whether l was created by [NewLevels].
func (l Levels[T]) Valid() bool { return len(l.levels) >= 2 }

// Len returns the number of levels.
func (l Levels[T]) Len() int { return len(l.levels) }

// At returns the level at index i, counting from the lowest, and whether i is in
// range.
func (l Levels[T]) At(i int) (Level[T], bool) {
	if i < 0 || i >= len(l.levels) {
		return Level[T]{}, false
	}
	return l.levels[i], true
}

// Names returns the level names, lowest first.
func (l Levels[T]) Names() []T {
	names := make([]T, len(l.levels))
	for i, level := range l.levels {
		names[i] = level.Name
	}
	return names
}

func (l Levels[T]) criteria() []string {
	criteria := make([]string, len(l.levels))
	for i, level := range l.levels {
		criteria[i] = level.Description
		if criteria[i] == "" {
			criteria[i] = string(level.Name)
		}
	}
	return criteria
}

func (l Levels[T]) sizeBytes() int {
	size := 0
	for _, level := range l.levels {
		size += len(level.Name) + len(level.Description)
	}
	return size
}
