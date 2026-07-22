package tests

import (
	"testing"

	"github.com/tinywasm/view"
	"github.com/tinywasm/view/conformance"
	"github.com/tinywasm/view/mock"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, conformance.Factory{
		New: func(t *testing.T, p view.Presenter) conformance.Driver {
			r := mock.New(p)
			return conformance.Driver{
				Mount:          r.Mount,
				Labels:         r.Labels,
				Select:         r.Select,
				SetField:       r.SetField,
				Save:           r.Save,
				Delete:         r.Delete,
				New:            r.New,
				Edit:           r.Edit,
				Cancel:         r.Cancel,
				FocusedFieldID: r.FocusedFieldID,
			}
		},
	})
}
