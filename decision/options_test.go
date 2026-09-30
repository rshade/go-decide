package decision

import (
	"errors"
	"slices"
	"testing"
)

type action string

const (
	ship action = "ship"
	hold action = "hold"
	kill action = "kill"
)

func TestNewOptions(t *testing.T) {
	tests := []struct {
		name    string
		in      map[action]string
		wantErr bool
	}{
		{"two options", map[action]string{ship: "release it", hold: "wait"}, false},
		{"three options", map[action]string{ship: "a", hold: "b", kill: "c"}, false},
		{"empty descriptions", map[action]string{ship: "", hold: ""}, false},
		{"nil map", nil, true},
		{"empty map", map[action]string{}, true},
		{"one option", map[action]string{ship: "release it"}, true},
		{"empty name", map[action]string{ship: "a", "": "b"}, true},
		{"only an empty name", map[action]string{"": "a"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o, err := NewOptions(tt.in)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidOptions) {
					t.Fatalf("NewOptions error = %v, want ErrInvalidOptions", err)
				}
				if o.Valid() {
					t.Fatal("NewOptions returned a valid set alongside an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewOptions unexpected error: %v", err)
			}
			if !o.Valid() || o.Len() != len(tt.in) {
				t.Fatalf("NewOptions = valid %v len %d, want valid len %d", o.Valid(), o.Len(), len(tt.in))
			}
			for name, desc := range tt.in {
				got, ok := o.Description(name)
				if !ok || got != desc || !o.Contains(name) {
					t.Errorf("option %q = %q, %v; want %q, true", name, got, ok, desc)
				}
			}
		})
	}
}

func TestOptionsAreCopiedAndOrdered(t *testing.T) {
	in := map[action]string{ship: "a", hold: "b", kill: "c"}
	o, err := NewOptions(in)
	if err != nil {
		t.Fatalf("NewOptions error: %v", err)
	}

	in[ship] = "changed"
	delete(in, hold)
	in["extra"] = "d"

	if got, _ := o.Description(ship); got != "a" {
		t.Errorf("description of ship = %q after the input changed, want a", got)
	}
	if !o.Contains(hold) || o.Contains("extra") {
		t.Errorf("options followed later changes to the input map")
	}
	if got, want := o.Names(), []action{hold, kill, ship}; !slices.Equal(got, want) {
		t.Errorf("Names() = %v, want sorted %v", got, want)
	}
}

func TestZeroOptionsAreNotValid(t *testing.T) {
	var unset Options[action]

	if unset.Valid() || unset.Len() != 0 || unset.Contains(ship) {
		t.Fatal("the zero Options must be invalid, empty and contain nothing")
	}
}
