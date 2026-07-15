package mock

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/model"
	"github.com/tinywasm/view"
)

// Renderer es un renderer headless (sin DOM) de referencia para pruebas y demostración.
// Sigue fielmente la especificación del presentador y sirve de puente en las pruebas de conformidad.
type Renderer struct {
	p    view.Presenter
	desc view.Descriptor
	form map[string]string
}

// New crea una instancia del renderer headless para un Descriptor dado.
func New(desc view.Descriptor) (*Renderer, error) {
	p, err := view.New(desc)
	if err != nil {
		return nil, err
	}
	return &Renderer{
		p:    p,
		desc: desc,
		form: make(map[string]string),
	}, nil
}

// Presenter expone el presentador interno para inspección o uso directo.
func (r *Renderer) Presenter() view.Presenter {
	return r.p
}

// Mount inicia la recarga de datos en el presentador.
func (r *Renderer) Mount() {
	r.p.Reload(nil)
}

// Labels devuelve las etiquetas de los elementos cargados en el presentador.
func (r *Renderer) Labels() []string {
	items := r.p.Items()
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = item.Label
	}
	return labels
}

// Select marca un id como seleccionado, carga su modelo completo en el formulario headless.
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

// SetField asigna un valor a un campo en el formulario headless.
func (r *Renderer) SetField(name, value string) {
	r.form[name] = value
}

// Save sincroniza todos los valores acumulados en el formulario headless en el Record
// del Descriptor, y luego llama a Presenter.Save para enviar los datos.
func (r *Renderer) Save() {
	rec := r.desc.Record
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
	r.p.Save(nil)
}

// Delete elimina el registro seleccionado.
func (r *Renderer) Delete() {
	r.p.Delete(r.p.Selected(), nil)
}
