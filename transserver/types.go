package main

import (
	"github.com/petert82/go-translation-api/trans"
)

type Domain struct {
	Name    string   `json:"name"`
	Strings []String `json:"strings"`
}

func NewDomain(dd trans.Domain) (d *Domain) {
	ds := dd.Strings()
	d = &Domain{Name: dd.Name(), Strings: make([]String, len(ds))}

	for i, s := range ds {
		ns := String{Name: s.Name(), Translations: make(map[string]Translation)}
		for l, t := range s.Translations() {
			ns.Translations[l.Code] = Translation{Content: t.Content()}
		}
		d.Strings[i] = ns
	}

	return d
}

type String struct {
	Name         string                 `json:"name"`
	Translations map[string]Translation `json:"translations"`
}

type Translation struct {
	Content string `json:"content"`
}
