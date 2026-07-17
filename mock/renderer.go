package mock

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/model"
	"github.com/tinywasm/view"
)

// Renderer is a headless reference renderer for browser-less simulation and tests.
type Renderer struct {
	p    view.Presenter
	form map[string]string
}

// New creates a reference renderer instance for a given Presenter.
func New(p view.Presenter) *Renderer {
	return &Renderer{
		p:    p,
		form: make(map[string]string),
	}
}

// Presenter returns the underlying presenter.
func (r *Renderer) Presenter() view.Presenter {
	return r.p
}

// Mount triggers loading data into the presenter synchronously.
func (r *Renderer) Mount() {
	_ = r.p.Reload()
}

// Labels returns labels of the loaded items.
func (r *Renderer) Labels() []string {
	items := r.p.Items()
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = item.Label
	}
	return labels
}

// Select marks id as selected, and populates r.form with the record's schema fields.
func (r *Renderer) Select(id string) {
	m := r.p.Select(id)
	r.form = make(map[string]string)
	if !model.IsNil(m) {
		schema := m.Schema()
		pointers := m.Pointers()
		for i, f := range schema {
			ptr := pointers[i]
			var val string
			switch p := ptr.(type) {
			case *string:
				val = *p
			case *int64:
				val = fmt.Sprintf("%d", *p)
			case *float64:
				val = fmt.Sprintf("%f", *p)
			case *bool:
				val = fmt.Sprintf("%t", *p)
			}
			r.form[f.Name] = val
		}
	}
}

// Deselect clears the selection and form.
func (r *Renderer) Deselect() {
	r.p.Deselect()
	r.form = make(map[string]string)
}

// Filter filters the presenter items and returns their labels.
func (r *Renderer) Filter(term string) []string {
	items := r.p.Filter(term)
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = item.Label
	}
	return labels
}

// SetField sets a form field value.
func (r *Renderer) SetField(name, value string) {
	r.form[name] = value
}

// Save synchronizes the headless form values with the Record, and calls Save.
func (r *Renderer) Save() {
	s, ok := r.p.(view.Saver)
	if !ok {
		return
	}
	rec := r.p.Record()
	if !model.IsNil(rec) {
		schema := rec.Schema()
		pointers := rec.Pointers()
		for i, f := range schema {
			if val, ok := r.form[f.Name]; ok {
				ptr := pointers[i]
				switch p := ptr.(type) {
				case *string:
					*p = val
				case *int64:
					var v int64
					_, _ = fmt.Sscanf(val, "%d", &v)
					*p = v
				case *float64:
					var v float64
					_, _ = fmt.Sscanf(val, "%f", &v)
					*p = v
				case *bool:
					if val == "true" || val == "1" {
						*p = true
					} else {
						*p = false
					}
				}
			}
		}
	}
	_ = s.Save(rec)
}

// Delete deletes the selected record.
func (r *Renderer) Delete() {
	d, ok := r.p.(view.Deleter)
	if !ok {
		return
	}
	_ = d.Delete(r.p.Selected())
}
