package view

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/json"
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
)

type presenter struct {
	caller            router.Caller
	record            model.Model
	listOp            string
	newList           func() model.FielderSlice
	project           func(list model.FielderSlice) []Item
	title             string
	searchPlaceholder string
	saveOp            string
	deleteOp          string
	args              func() model.Encodable
	fill              func(id string) model.Model

	items    []Item
	selected string
}

func (p *presenter) Title() string {
	return p.title
}

func (p *presenter) SearchPlaceholder() string {
	return p.searchPlaceholder
}

func (p *presenter) Record() model.Model {
	return p.record
}

func (p *presenter) Items() []Item {
	return p.items
}

func (p *presenter) Selected() string {
	return p.selected
}

func (p *presenter) Reload() error {
	var listArgs model.Encodable
	if p.args != nil {
		listArgs = p.args()
	}

	ch := make(chan error, 1)
	var rawResult []byte

	p.caller.Call(p.listOp, listArgs, func(result []byte, err error) {
		rawResult = result
		ch <- err
	})

	if err := <-ch; err != nil {
		return err
	}

	list := p.newList()
	dec, ok := list.(model.Decodable)
	if !ok {
		return fmt.Err("view: list returned by newList does not implement model.Decodable")
	}

	if err := json.Decode(rawResult, dec); err != nil {
		return err
	}

	p.items = p.project(list)
	return nil
}

func (p *presenter) Select(id string) model.Model {
	if id == "" {
		p.selected = ""
		if p.fill != nil {
			p.fill("")
		}
		return nil
	}
	p.selected = id
	if p.fill != nil {
		return p.fill(id)
	}
	return nil
}

func (p *presenter) CanSave() bool {
	return p.saveOp != ""
}

func (p *presenter) Save(payload model.Model) error {
	if !p.CanSave() {
		return fmt.Err("view: Save not allowed (SaveOp is empty)")
	}
	if model.IsNil(payload) {
		return fmt.Err("view: Save payload is nil")
	}

	ch := make(chan error, 1)
	p.caller.Call(p.saveOp, payload, func(result []byte, err error) {
		ch <- err
	})
	return <-ch
}

func (p *presenter) CanDelete() bool {
	return p.deleteOp != ""
}

func (p *presenter) Delete(id string) error {
	if !p.CanDelete() {
		return fmt.Err("view: Delete not allowed (DeleteOp is empty)")
	}

	var rec model.Model
	if p.fill != nil {
		rec = p.fill(id)
	}

	ch := make(chan error, 1)
	p.caller.Call(p.deleteOp, rec, func(result []byte, err error) {
		ch <- err
	})
	return <-ch
}
