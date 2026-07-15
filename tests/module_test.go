package tests

import (
	"testing"

	"github.com/tinywasm/model"
	"github.com/tinywasm/router/mock"
	"github.com/tinywasm/view"
	"github.com/tinywasm/view/conformance"
)

func TestModulePerspective(t *testing.T) {
	caller := &mock.Caller{
		CannedResult: []byte(`[{"id":"m1","name":"Module 1"},{"id":"m2","name":"Module 2"}]`),
	}
	record := &conformance.MockRecord{}
	cache := map[string]*conformance.MockRecord{
		"m1": {ID: "m1", Name: "Module 1"},
		"m2": {ID: "m2", Name: "Module 2"},
	}

	desc := view.Descriptor{
		Title:    "Module View",
		Record:   record,
		Caller:   caller,
		ListOp:   "list_items",
		SaveOp:   "save_item",
		DeleteOp: "delete_item",
		NewList: func() model.FielderSlice {
			return &conformance.MockList{}
		},
		Project: func(list model.FielderSlice) []view.Item {
			l := list.(*conformance.MockList)
			items := make([]view.Item, l.Len())
			for i := 0; i < l.Len(); i++ {
				mr := l.At(i).(*conformance.MockRecord)
				items[i] = view.Item{ID: mr.ID, Label: mr.Name}
			}
			return items
		},
		Fill: func(id string) model.Model {
			if id == "" {
				return nil
			}
			return cache[id]
		},
	}

	p, err := view.New(desc)
	if err != nil {
		t.Fatalf("failed to build presenter: %v", err)
	}

	// 1. Reload -> ListOp
	calledReload := false
	p.Reload(func(err error) {
		if err != nil {
			t.Fatalf("reload failed: %v", err)
		}
		calledReload = true
	})
	if !calledReload {
		t.Errorf("expected reload callback to be invoked")
	}

	items := p.Items()
	if len(items) != 2 || items[0].ID != "m1" || items[1].ID != "m2" {
		t.Errorf("unexpected items: %v", items)
	}

	// 2. Select / Selected
	if p.Selected() != "" {
		t.Errorf("expected initially empty selection")
	}

	m := p.Select("m2")
	if m == nil {
		t.Errorf("expected selected model to be returned")
	} else {
		mr := m.(*conformance.MockRecord)
		if mr.ID != "m2" || mr.Name != "Module 2" {
			t.Errorf("unexpected model fields: %v", mr)
		}
	}
	if p.Selected() != "m2" {
		t.Errorf("expected Selected() to be 'm2', got %q", p.Selected())
	}

	// 3. Save -> SaveOp with Record
	calledSave := false
	p.Save(func(err error) {
		if err != nil {
			t.Fatalf("save failed: %v", err)
		}
		calledSave = true
	})
	if !calledSave {
		t.Errorf("expected save callback to be invoked")
	}

	var savedRecord *conformance.MockRecord
	for _, call := range caller.Calls {
		if call.Op == "save_item" {
			savedRecord = call.Args.(*conformance.MockRecord)
		}
	}
	if savedRecord != record {
		t.Errorf("expected save payload to be exactly Descriptor.Record, got %v", savedRecord)
	}

	// 4. Delete -> DeleteOp with record from Fill
	calledDelete := false
	p.Delete("m1", func(err error) {
		if err != nil {
			t.Fatalf("delete failed: %v", err)
		}
		calledDelete = true
	})
	if !calledDelete {
		t.Errorf("expected delete callback to be invoked")
	}

	var deletedRecord *conformance.MockRecord
	for _, call := range caller.Calls {
		if call.Op == "delete_item" {
			deletedRecord = call.Args.(*conformance.MockRecord)
		}
	}
	if deletedRecord == nil || deletedRecord.ID != "m1" || deletedRecord.Name != "Module 1" {
		t.Errorf("expected delete payload to represent 'm1', got %v", deletedRecord)
	}

	// 5. Select("") -> nil
	p.Select("")
	if p.Selected() != "" {
		t.Errorf("expected selected to be cleared")
	}
}
