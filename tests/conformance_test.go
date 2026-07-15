package tests

import (
	"testing"

	"github.com/tinywasm/view"
	"github.com/tinywasm/view/conformance"
	"github.com/tinywasm/view/mock"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, conformance.Factory{
		New: func(t *testing.T, desc view.Descriptor) conformance.Driver {
			r, err := mock.New(desc)
			if err != nil {
				t.Fatalf("failed to create mock renderer: %v", err)
			}
			return conformance.Driver{
				Mount:    r.Mount,
				Labels:   r.Labels,
				Select:   r.Select,
				SetField: r.SetField,
				Save:     r.Save,
				Delete:   r.Delete,
			}
		},
	})
}
