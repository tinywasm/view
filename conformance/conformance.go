package conformance

import (
	"testing"

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

// Factory construye el renderer bajo prueba alrededor del presenter y devuelve un Driver que simula la
// interacción del usuario con él. Es el seam específico de tecnología (como ServeFunc en router).
type Factory struct {
	New func(t *testing.T, p view.Presenter) Driver
}

// Driver simula los eventos de UI sobre el renderer, sin que la suite conozca su tecnología.
type Driver struct {
	Mount    func()                    // provoca el init: el renderer carga la lista
	Labels   func() []string           // lo que la lista muestra ahora
	Select   func(id string)           // simula seleccionar la fila con ese id
	SetField func(name, value string)  // fija un campo del form
	Save     func()                    // simula la acción guardar
	Delete   func()                    // simula la acción eliminar
}

// MockRecord es un modelo de simulación de un registro para el arnés de conformidad.
type MockRecord struct {
	ID   string
	Name string
}

// ModelName implementa model.ModuleNaming.
func (m *MockRecord) ModelName() string { return "MockRecord" }

// IsNil implementa model.Model.
func (m *MockRecord) IsNil() bool { return m == nil }

// Schema implementa model.Fielder.
func (m *MockRecord) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: model.Text()},
		{Name: "name", Type: model.Text()},
	}
}

// Pointers implementa model.Fielder.
func (m *MockRecord) Pointers() []any {
	return []any{&m.ID, &m.Name}
}

// EncodeFields implementa model.Encodable.
func (m *MockRecord) EncodeFields(w model.FieldWriter) {
	w.String("id", m.ID)
	w.String("name", m.Name)
}

// DecodeFields implementa model.Decodable.
func (m *MockRecord) DecodeFields(r model.FieldReader) {
	if val, ok := r.String("id"); ok {
		m.ID = val
	}
	if val, ok := r.String("name"); ok {
		m.Name = val
	}
}

// MockList es un tipo de lista que contiene registros de simulación.
type MockList struct {
	items []*MockRecord
}

// IsNil implementa model.Model.
func (m *MockList) IsNil() bool { return m == nil }

// DecodeFields implementa model.Decodable.
func (m *MockList) DecodeFields(r model.FieldReader) {}

// Schema implementa model.Fielder.
func (m *MockList) Schema() []model.Field { return nil }

// Pointers implementa model.Fielder.
func (m *MockList) Pointers() []any { return nil }

// Len implementa model.FielderSlice.
func (m *MockList) Len() int { return len(m.items) }

// At implementa model.FielderSlice.
func (m *MockList) At(i int) model.Fielder {
	return m.items[i]
}

// Append implementa model.FielderSlice.
func (m *MockList) Append() model.Fielder {
	it := &MockRecord{}
	m.items = append(m.items, it)
	return it
}

// Run ejecuta el conjunto completo de cláusulas de conformidad del renderer.
func Run(t *testing.T, f Factory) {
	t.Run("mount_triggers_list_load", func(t *testing.T) {
		caller := &FakeCaller{}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.FielderSlice { return &MockList{} },
			func(list model.FielderSlice) []view.Item { return nil },
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
			func() model.FielderSlice { return &MockList{} },
			func(list model.FielderSlice) []view.Item {
				l := list.(*MockList)
				items := make([]view.Item, l.Len())
				for i := 0; i < l.Len(); i++ {
					mr := l.At(i).(*MockRecord)
					items[i] = view.Item{ID: mr.ID, Label: mr.Name}
				}
				return items
			},
		)

		driver := f.New(t, p)
		driver.Mount()

		labels := driver.Labels()
		if len(labels) != 2 || labels[0] != "Alice" || labels[1] != "Bob" {
			t.Errorf("expected labels %v, got %v", []string{"Alice", "Bob"}, labels)
		}
	})

	t.Run("select_fills_form", func(t *testing.T) {
		caller := &FakeCaller{}
		record := &MockRecord{}
		cache := map[string]*MockRecord{
			"1": {ID: "1", Name: "Alice"},
			"2": {ID: "2", Name: "Bob"},
		}

		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.FielderSlice { return &MockList{} },
			func(list model.FielderSlice) []view.Item {
				return []view.Item{
					{ID: "1", Label: "Alice"},
					{ID: "2", Label: "Bob"},
				}
			},
			view.WithSaveOp("test_save_op"),
			view.WithDeleteOp("test_delete_op"),
			view.WithFill(func(id string) model.Model {
				if id == "" {
					return nil
				}
				return cache[id]
			}),
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
			func() model.FielderSlice { return &MockList{} },
			func(list model.FielderSlice) []view.Item { return nil },
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
		caller := &FakeCaller{}
		record := &MockRecord{}
		cache := map[string]*MockRecord{
			"1": {ID: "1", Name: "Alice"},
			"2": {ID: "2", Name: "Bob"},
		}

		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.FielderSlice { return &MockList{} },
			func(list model.FielderSlice) []view.Item {
				return []view.Item{
					{ID: "1", Label: "Alice"},
					{ID: "2", Label: "Bob"},
				}
			},
			view.WithDeleteOp("test_delete_op"),
			view.WithFill(func(id string) model.Model {
				if id == "" {
					return nil
				}
				return cache[id]
			}),
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
			func() model.FielderSlice { return &MockList{} },
			func(list model.FielderSlice) []view.Item { return nil },
			// SaveOp left empty
		)

		driver := f.New(t, p)
		driver.Mount()
		driver.Save()

		for _, call := range caller.Calls {
			if call.Op == "" || call.Op == "test_save_op" {
				t.Errorf("did not expect any save calls, but got call to %q", call.Op)
			}
		}
	})
}
