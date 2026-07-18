package view

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
)

type core struct {
	caller            router.Caller
	record            model.Model
	listOp            string
	newList           func() model.ModelSlice
	title             string
	searchPlaceholder string
	saveOp            string
	deleteOp          string
	args              func() model.Encodable

	items    []Item
	selected string
	index    map[string]model.Model
}

func (p *core) Title() string {
	return p.title
}

func (p *core) SearchPlaceholder() string {
	return p.searchPlaceholder
}

func (p *core) Record() model.Model {
	return p.record
}

func (p *core) Items() []Item {
	return p.items
}

func (p *core) Selected() string {
	return p.selected
}

func (p *core) Reload() error {
	var listArgs model.Encodable
	if p.args != nil {
		listArgs = p.args()
	}

	list := p.newList()
	dec, ok := list.(model.Decodable)
	if !ok {
		return fmt.Err("view: list returned by newList does not implement model.Decodable")
	}

	ch := make(chan error, 1)
	p.caller.Call(p.listOp, listArgs, dec, func(err error) { ch <- err })
	if err := <-ch; err != nil {
		return err
	}

	p.items = p.items[:0]
	p.index = make(map[string]model.Model, list.Len())
	for i := 0; i < list.Len(); i++ {
		row := list.At(i)
		iz, ok := row.(Itemizer)
		if !ok {
			return fmt.Err("view: Reload: row type", rowName(row), "does not implement view.Itemizer")
		}
		m, ok := row.(model.Model)
		if !ok {
			return fmt.Err("view: Reload: row type", rowName(row), "does not implement model.Model")
		}
		it := iz.Item()
		p.items = append(p.items, it)
		p.index[it.ID] = m
	}

	return nil
}

// rowName names the row that failed an Itemizer/model.Model assertion in Reload,
// so the error points at the offending record. tinywasm/fmt has no reflect-based
// type-name formatter (WASM-size discipline), so this uses model.ModuleNaming —
// the stable name ormc already generates for every domain record — when the row
// provides it.
func rowName(row model.Fielder) string {
	if mn, ok := row.(model.ModuleNaming); ok {
		return mn.ModelName()
	}
	return "<unnamed type>"
}

func (p *core) Select(id string) model.Model {
	m, ok := p.index[id]
	if !ok {
		return nil
	}
	p.selected = id
	return m
}

func (p *core) Deselect() {
	p.selected = ""
}

func (p *core) Filter(term string) []Item {
	if term == "" {
		res := make([]Item, len(p.items))
		copy(res, p.items)
		return res
	}
	var filtered []Item
	for _, it := range p.items {
		if fmt.Matches(it.Label, term) || fmt.Matches(it.Description, term) {
			filtered = append(filtered, it)
		}
	}
	return filtered
}

type saveable struct {
	*core
}

func (s *saveable) Save(payload model.Model) error {
	if model.IsNil(payload) {
		return fmt.Err("view: Save payload is nil")
	}

	ch := make(chan error, 1)
	s.caller.Call(s.saveOp, payload, nil, func(err error) { ch <- err })
	return <-ch
}

type deletable struct {
	*core
}

func (d *deletable) Delete(id string) error {
	rec, ok := d.index[id]
	if !ok {
		return fmt.Err("view: Delete: unknown id")
	}

	ch := make(chan error, 1)
	d.caller.Call(d.deleteOp, rec, nil, func(err error) { ch <- err })
	return <-ch
}

type crud struct {
	*core
}

func (c *crud) Save(payload model.Model) error {
	if model.IsNil(payload) {
		return fmt.Err("view: Save payload is nil")
	}

	ch := make(chan error, 1)
	c.caller.Call(c.saveOp, payload, nil, func(err error) { ch <- err })
	return <-ch
}

func (c *crud) Delete(id string) error {
	rec, ok := c.index[id]
	if !ok {
		return fmt.Err("view: Delete: unknown id")
	}

	ch := make(chan error, 1)
	c.caller.Call(c.deleteOp, rec, nil, func(err error) { ch <- err })
	return <-ch
}
