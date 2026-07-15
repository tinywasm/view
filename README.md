# view

Tech-agnostic CRUD view contract: a module declares its `Descriptor`, and any renderer draws it.

## Design Goals

1. **Agnostic to UI Technology**: The domain module declares its intention, schema, operations, and list projections without importing any specific UI package (such as `dom`, `form`, or `html`). This allows changing renderers seamlessly (e.g. standard layout, HTMX, Native UI, SSR) without touching any business/domain module.
2. **Unit Testable Business Logic**: By separating the presenter logic from the rendering logic (Model-View-Presenter / ViewModel), the view's logical flow can be tested end-to-end without a browser or a DOM, simply utilizing test doubles (like `router/mock.Caller`).
3. **Codec-Free Modules**: The domain module never imports `json` or any wire encoding library; decoding raw payload bytes into typed models is done within the `view` infrastructure, keeping domain models completely clean of codec dependencies.

## Key Concepts

### `Descriptor`
Declared by the business/domain module. It defines the logical fields, operations, and callbacks that govern the view:
* **`Title`**: View header.
* **`Record`**: The shared model that the form targets and synchronized input gets saved into.
* **`Caller`**: The network/logical router transportation boundary.
* **`ListOp`**, **`SaveOp`**, **`DeleteOp`**: Logical operation names invoked on the caller.
* **`Project`**: Converts decoded lists into neutral list elements (`Item`) and populates lookup caches.
* **`Fill`**: Resolves full domain models from selection IDs.

### `Presenter`
The state machine engine built by `view.New(Descriptor)`. It handles all interactions and state changes (Reload, Select, Save, Delete) and updates `Items` and `Selected` ID state.

---

## Quick Start

### 1. Declaring a View (Domain Module)

```go
package catalog

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
	"github.com/tinywasm/view"
)

func NewCatalogView(caller router.Caller) (view.Presenter, error) {
	record := &CatalogItem{}
	var cache = make(map[string]*CatalogItem)

	return view.New(view.Descriptor{
		Title:  "Catalog Management",
		Record: record,
		Caller: caller,
		ListOp: "catalog.list",
		SaveOp: "catalog.save",
		Args:   func() model.Encodable { return nil },
		NewList: func() model.FielderSlice {
			return &CatalogItemList{}
		},
		Project: func(list model.FielderSlice) []view.Item {
			l := list.(*CatalogItemList)
			items := make([]view.Item, l.Len())
			for i := 0; i < l.Len(); i++ {
				item := l.At(i).(*CatalogItem)
				items[i] = view.Item{
					ID:          item.ID,
					Label:       item.Name,
					Description: item.SKU,
				}
				cache[item.ID] = item
			}
			return items
		},
		Fill: func(id string) model.Model {
			if id == "" {
				return nil
			}
			return cache[id]
		},
	})
}
```

### 2. Wrapping with a Renderer (UI Layer)

Any concrete UI renderer (like `layout/crudview`) wraps the `Presenter`, generates form inputs from `Record.Schema()`, synchronizes user input back into `Record` before calling `Save()`, and renders the list items:

```go
package crudview

import (
	"github.com/tinywasm/view"
)

type Renderer struct {
	p    view.Presenter
	desc view.Descriptor
}

func (r *Renderer) OnSaveClicked() {
	// 1. Sync DOM/form inputs into r.desc.Record (validated)
	// 2. Instruct presenter to save
	r.p.Save(func(err error) {
		if err != nil {
			r.ShowError(err)
		}
	})
}
```

## Reference and Conformance

* **`view/mock`**: Package containing a reference headless renderer (`mock.Renderer`) useful for browser-less simulations and unit testing.
* **`view/conformance`**: Test arnes package that exports `conformance.Run(t, Factory)`. Any prospective renderer must satisfy the 6 conformance clauses to be considered correct.
