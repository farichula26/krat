package main

import (
	"reflect"
	"testing"
)

func TestSplitter(t *testing.T) {
	cases := []struct {
		name   string
		deltas []string
		want   []string
	}{
		{"two sentences streamed", []string{"Hola, veo una taza ", "roja en tu mano. ¿Es ", "tuya o de alguien más?"},
			[]string{"Hola, veo una taza roja en tu mano.", "¿Es tuya o de alguien más?"}},
		{"short fragment merges", []string{"Sí. Claro que te ayudo con eso. Dime."},
			[]string{"Sí. Claro que te ayudo con eso.", "Dime."}},
		{"decimal is not a boundary", []string{"Cuesta 3.5 euros más o menos, creo yo."},
			[]string{"Cuesta 3.5 euros más o menos, creo yo."}},
		{"newline boundary and ellipsis", []string{"Déjame pensarlo un poco…\nVale, ya lo tengo claro."},
			[]string{"Déjame pensarlo un poco…", "Vale, ya lo tengo claro."}},
		{"empty", []string{""}, nil},
	}
	for _, c := range cases {
		var s splitter
		var got []string
		for _, d := range c.deltas {
			got = append(got, s.Push(d)...)
		}
		got = append(got, s.Flush()...)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
