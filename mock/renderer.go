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
	form map[string]string
}

// New crea una instancia del renderer headless para un Presenter dado.
func New(p view.Presenter) *Renderer {
	return &Renderer{
		p:    p,
		form: make(map[string]string),
	}
}

// Presenter expone el presentador interno para inspección o uso directo.
func (r *Renderer) Presenter() view.Presenter {
	return r.p
}

// Mount inicia la recarga de datos en el presentador de forma síncrona.
func (r *Renderer) Mount() {
	_ = r.p.Reload()
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
// del Presenter, y luego llama a Presenter.Save pasándole el payload explícito.
func (r *Renderer) Save() {
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
	_ = r.p.Save(rec)
}

// Delete elimina el registro seleccionado.
func (r *Renderer) Delete() {
	_ = r.p.Delete(r.p.Selected())
}
