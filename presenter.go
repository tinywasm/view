package view

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/json"
	"github.com/tinywasm/model"
)

type presenter struct {
	d        Descriptor
	items    []Item
	selected string
}

func (p *presenter) Items() []Item {
	return p.items
}

func (p *presenter) Reload(done func(error)) {
	var args model.Encodable
	if p.d.Args != nil {
		args = p.d.Args()
	}

	p.d.Caller.Call(p.d.ListOp, args, func(result []byte, err error) {
		if err != nil {
			if done != nil {
				done(err)
			}
			return
		}

		list := p.d.NewList()
		dec, ok := list.(model.Decodable)
		if !ok {
			if done != nil {
				done(fmt.Err("view: list returned by NewList does not implement model.Decodable"))
			}
			return
		}

		if err := json.Decode(result, dec); err != nil {
			if done != nil {
				done(err)
			}
			return
		}

		p.items = p.d.Project(list)
		if done != nil {
			done(nil)
		}
	})
}

func (p *presenter) Selected() string {
	return p.selected
}

func (p *presenter) Select(id string) model.Model {
	if id == "" {
		p.selected = ""
		if p.d.Fill != nil {
			p.d.Fill("")
		}
		return nil
	}
	p.selected = id
	if p.d.Fill != nil {
		return p.d.Fill(id)
	}
	return nil
}

func (p *presenter) CanSave() bool {
	return p.d.SaveOp != ""
}

func (p *presenter) Save(done func(error)) {
	if !p.CanSave() {
		if done != nil {
			done(fmt.Err("view: Save not allowed (SaveOp is empty)"))
		}
		return
	}

	p.d.Caller.Call(p.d.SaveOp, p.d.Record, func(result []byte, err error) {
		if done != nil {
			done(err)
		}
	})
}

func (p *presenter) CanDelete() bool {
	return p.d.DeleteOp != ""
}

func (p *presenter) Delete(id string, done func(error)) {
	if !p.CanDelete() {
		if done != nil {
			done(fmt.Err("view: Delete not allowed (DeleteOp is empty)"))
		}
		return
	}

	var record model.Model
	if p.d.Fill != nil {
		record = p.d.Fill(id)
	}

	p.d.Caller.Call(p.d.DeleteOp, record, func(result []byte, err error) {
		if done != nil {
			done(err)
		}
	})
}
