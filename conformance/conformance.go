package conformance

import (
	"testing"

	"github.com/tinywasm/form/input"
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
	"github.com/tinywasm/view"
)

// FakeCall records one invocation the suite (or a consumer's module test) can inspect.
type FakeCall struct {
	Op   string
	Args model.Encodable
}

// FakeCaller is a codec-free router.Caller test double. Unlike router/mock.Caller it does
// NOT decode a wire response — it fills the typed target directly via Reply — so this
// package (and any renderer or module that imports it for tests) depends only on model and
// router, never a codec. That is the same discipline router/conformance keeps: the arnés of
// a contract must not drag in an implementation.
type FakeCaller struct {
	Calls []FakeCall
	// Reply fills `into` with canned TYPED data for op (no serialization); nil = no result.
	Reply func(op string, into model.Decodable)
	// Err, if set, is what every Call reports (and suppresses Reply).
	Err error
}

func (c *FakeCaller) Call(op string, args model.Encodable, into model.Decodable, done func(err error)) {
	c.Calls = append(c.Calls, FakeCall{Op: op, Args: args})
	if c.Err == nil && c.Reply != nil && into != nil {
		c.Reply(op, into)
	}
	if done != nil {
		done(c.Err)
	}
}

func (c *FakeCaller) Dispatch(op string, args model.Encodable) {
	c.Calls = append(c.Calls, FakeCall{Op: op, Args: args})
}

var _ router.Caller = (*FakeCaller)(nil)

// Factory builds the renderer under test around the presenter and returns a Driver.
type Factory struct {
	New func(t *testing.T, p view.Presenter) Driver
}

// Driver simulates user UI interaction over the renderer.
type Driver struct {
	Mount    func()                   // triggers initialization: renderer loads list
	Labels   func() []string          // what the list shows right now
	Select   func(id string)          // simulates picking a row with that id
	SetField func(name, value string) // sets a form field
	Save     func()                   // simulates the save action
	Delete   func()                   // simulates the delete action
}

// MockRecord is a simulation record for conformance suite.
type MockRecord struct {
	ID   string
	Name string
}

// ModelName implements model.ModuleNaming.
func (m *MockRecord) ModelName() string { return "MockRecord" }

// IsNil implements model.Model.
func (m *MockRecord) IsNil() bool { return m == nil }

// Schema implements model.Fielder.
func (m *MockRecord) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: input.Text()},
		{Name: "name", Type: input.Text()},
	}
}

// Pointers implements model.Fielder.
func (m *MockRecord) Pointers() []any {
	return []any{&m.ID, &m.Name}
}

// EncodeFields implements model.Encodable.
func (m *MockRecord) EncodeFields(w model.FieldWriter) {
	w.String("id", m.ID)
	w.String("name", m.Name)
}

// DecodeFields implements model.Decodable.
func (m *MockRecord) DecodeFields(r model.FieldReader) {
	if val, ok := r.String("id"); ok {
		m.ID = val
	}
	if val, ok := r.String("name"); ok {
		m.Name = val
	}
}

// Item implements view.Itemizer.
func (m *MockRecord) Item() view.Item {
	return view.Item{
		ID:          m.ID,
		Label:       m.Name,
		Description: "Desc of " + m.Name,
	}
}

// MockList is a list holding simulation records.
type MockList struct {
	items []*MockRecord
}

// IsNil implements model.Model.
func (m *MockList) IsNil() bool { return m == nil }

// DecodeFields implements model.Decodable.
func (m *MockList) DecodeFields(r model.FieldReader) {}

// Schema implements model.Fielder.
func (m *MockList) Schema() []model.Field { return nil }

// Pointers implements model.Fielder.
func (m *MockList) Pointers() []any { return nil }

// Len implements model.FielderSlice.
func (m *MockList) Len() int { return len(m.items) }

// At implements model.FielderSlice.
func (m *MockList) At(i int) model.Fielder {
	return m.items[i]
}

// Append implements model.FielderSlice.
func (m *MockList) Append() model.Fielder {
	it := &MockRecord{}
	m.items = append(m.items, it)
	return it
}

// Run executes the full set of conformance clauses.
func Run(t *testing.T, f Factory) {
	t.Run("mount_triggers_list_load", func(t *testing.T) {
		caller := &FakeCaller{}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
		)

		driver := f.New(t, p)
		driver.Mount()

		found := false
		for _, call := range caller.Calls {
			if call.Op == "test_list_op" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected call to %q, but was not found in %v", "test_list_op", caller.Calls)
		}
	})

	t.Run("list_renders_item_labels", func(t *testing.T) {
		caller := &FakeCaller{
			Reply: func(op string, into model.Decodable) {
				l := into.(*MockList)
				a := l.Append().(*MockRecord)
				a.ID, a.Name = "1", "Alice"
				b := l.Append().(*MockRecord)
				b.ID, b.Name = "2", "Bob"
			},
		}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
		)

		driver := f.New(t, p)
		driver.Mount()

		labels := driver.Labels()
		if len(labels) != 2 || labels[0] != "Alice" || labels[1] != "Bob" {
			t.Errorf("expected labels %v, got %v", []string{"Alice", "Bob"}, labels)
		}
	})

	t.Run("select_fills_form", func(t *testing.T) {
		caller := &FakeCaller{
			Reply: func(op string, into model.Decodable) {
				if op == "test_list_op" {
					l := into.(*MockList)
					a := l.Append().(*MockRecord)
					a.ID, a.Name = "1", "Alice"
					b := l.Append().(*MockRecord)
					b.ID, b.Name = "2", "Bob"
				}
			},
		}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
			view.WithSaveOp("test_save_op"),
			view.WithDeleteOp("test_delete_op"),
		)

		driver := f.New(t, p)
		driver.Mount()
		driver.Select("2")
		driver.Save()

		var savedRecord *MockRecord
		for _, call := range caller.Calls {
			if call.Op == "test_save_op" {
				if mr, ok := call.Args.(*MockRecord); ok {
					savedRecord = mr
				}
			}
		}

		if savedRecord == nil {
			t.Errorf("expected a save call with MockRecord payload")
		} else if savedRecord.ID != "2" || savedRecord.Name != "Bob" {
			t.Errorf("expected saved record to be ID '2' Name 'Bob', got ID %q Name %q", savedRecord.ID, savedRecord.Name)
		}
	})

	t.Run("save_ships_form_values", func(t *testing.T) {
		caller := &FakeCaller{}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
			view.WithSaveOp("test_save_op"),
		)

		driver := f.New(t, p)
		driver.Mount()
		driver.SetField("name", "X")
		driver.Save()

		var savedRecord *MockRecord
		for _, call := range caller.Calls {
			if call.Op == "test_save_op" {
				if mr, ok := call.Args.(*MockRecord); ok {
					savedRecord = mr
				}
			}
		}

		if savedRecord == nil {
			t.Fatalf("expected a save call with MockRecord payload")
		}
		if savedRecord.Name != "X" {
			t.Errorf("expected saved record to have Name 'X', got %q", savedRecord.Name)
		}
	})

	t.Run("delete_ships_selected_record", func(t *testing.T) {
		caller := &FakeCaller{
			Reply: func(op string, into model.Decodable) {
				if op == "test_list_op" {
					l := into.(*MockList)
					a := l.Append().(*MockRecord)
					a.ID, a.Name = "1", "Alice"
					b := l.Append().(*MockRecord)
					b.ID, b.Name = "2", "Bob"
				}
			},
		}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
			view.WithDeleteOp("test_delete_op"),
		)

		driver := f.New(t, p)
		driver.Mount()
		driver.Select("2")
		driver.Delete()

		var deletedRecord *MockRecord
		for _, call := range caller.Calls {
			if call.Op == "test_delete_op" {
				if mr, ok := call.Args.(*MockRecord); ok {
					deletedRecord = mr
				}
			}
		}

		if deletedRecord == nil {
			t.Errorf("expected a delete call with MockRecord payload")
		} else if deletedRecord.ID != "2" || deletedRecord.Name != "Bob" {
			t.Errorf("expected deleted record to be ID '2' Name 'Bob', got ID %q Name %q", deletedRecord.ID, deletedRecord.Name)
		}
	})

	t.Run("no_save_capability_when_saveop_empty", func(t *testing.T) {
		caller := &FakeCaller{}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
		)

		if _, ok := p.(view.Saver); ok {
			t.Errorf("expected presenter to not implement view.Saver when WithSaveOp is empty")
		}
	})

	t.Run("deselect_clears_selection", func(t *testing.T) {
		caller := &FakeCaller{
			Reply: func(op string, into model.Decodable) {
				if op == "test_list_op" {
					l := into.(*MockList)
					a := l.Append().(*MockRecord)
					a.ID, a.Name = "1", "Alice"
				}
			},
		}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
		)

		driver := f.New(t, p)
		driver.Mount()
		driver.Select("1")
		if p.Selected() != "1" {
			t.Errorf("expected selection to be '1', got %q", p.Selected())
		}
		p.Deselect()
		if p.Selected() != "" {
			t.Errorf("expected selection to be cleared, got %q", p.Selected())
		}
	})

	t.Run("select_unknown_id_returns_nil", func(t *testing.T) {
		caller := &FakeCaller{
			Reply: func(op string, into model.Decodable) {
				if op == "test_list_op" {
					l := into.(*MockList)
					a := l.Append().(*MockRecord)
					a.ID, a.Name = "1", "Alice"
				}
			},
		}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
		)

		driver := f.New(t, p)
		driver.Mount()
		driver.Select("1")
		if p.Selected() != "1" {
			t.Errorf("expected selection to be '1', got %q", p.Selected())
		}
		res := p.Select("unknown")
		if res != nil {
			t.Errorf("expected Select(unknown) to return nil, got %v", res)
		}
		if p.Selected() != "1" {
			t.Errorf("expected selection to remain '1', got %q", p.Selected())
		}
	})

	t.Run("filter_matches_label_and_description", func(t *testing.T) {
		caller := &FakeCaller{
			Reply: func(op string, into model.Decodable) {
				if op == "test_list_op" {
					l := into.(*MockList)
					a := l.Append().(*MockRecord)
					a.ID, a.Name = "1", "Alice" // description will be "Desc of Alice"
					b := l.Append().(*MockRecord)
					b.ID, b.Name = "2", "Bob"   // description will be "Desc of Bob"
				}
			},
		}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
		)

		driver := f.New(t, p)
		driver.Mount()

		// Case-insensitive match on label:
		res := p.Filter("alice")
		if len(res) != 1 || res[0].ID != "1" {
			t.Errorf("expected 1 result matching 'alice', got %v", res)
		}

		// Case-insensitive match on description:
		res = p.Filter("bOb")
		if len(res) != 1 || res[0].ID != "2" {
			t.Errorf("expected 1 result matching 'bOb', got %v", res)
		}

		// Empty term returns all:
		res = p.Filter("")
		if len(res) != 2 {
			t.Errorf("expected all 2 results on empty term, got %v", res)
		}
	})

	t.Run("delete_unknown_id_errors", func(t *testing.T) {
		caller := &FakeCaller{}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
			view.WithDeleteOp("test_delete_op"),
		)

		d, ok := p.(view.Deleter)
		if !ok {
			t.Fatalf("expected presenter to implement view.Deleter")
		}

		err := d.Delete("unknown")
		if err == nil {
			t.Errorf("expected error deleting unknown id, got nil")
		}

		for _, call := range caller.Calls {
			if call.Op == "test_delete_op" {
				t.Errorf("unexpected delete op call on unknown ID")
			}
		}
	})

	t.Run("no_delete_capability_when_deleteop_empty", func(t *testing.T) {
		caller := &FakeCaller{}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
		)

		if _, ok := p.(view.Deleter); ok {
			t.Errorf("expected presenter to not implement view.Deleter when WithDeleteOp is empty")
		}
	})
}
