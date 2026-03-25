package ast

import "testing"

func TestResolveExtensibility(t *testing.T) {
	tests := []struct {
		name   string
		annots []Annotation
		want   string
	}{
		{
			"no annotation defaults to APPENDABLE per XTypes 1.3",
			nil,
			ExtAppendable,
		},
		{
			"@final",
			[]Annotation{{Name: "final"}},
			ExtFinal,
		},
		{
			"@appendable",
			[]Annotation{{Name: "appendable"}},
			ExtAppendable,
		},
		{
			"@mutable",
			[]Annotation{{Name: "mutable"}},
			ExtMutable,
		},
		{
			"@extensibility(FINAL)",
			[]Annotation{{Name: "extensibility", Params: map[string]string{"value": "FINAL"}}},
			ExtFinal,
		},
		{
			"@extensibility(APPENDABLE)",
			[]Annotation{{Name: "extensibility", Params: map[string]string{"value": "APPENDABLE"}}},
			ExtAppendable,
		},
		{
			"@extensibility(MUTABLE)",
			[]Annotation{{Name: "extensibility", Params: map[string]string{"value": "MUTABLE"}}},
			ExtMutable,
		},
		{
			"positional param @extensibility(FINAL)",
			[]Annotation{{Name: "extensibility", Params: map[string]string{"": "FINAL"}}},
			ExtFinal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveExtensibility(tt.annots)
			if got != tt.want {
				t.Errorf("ResolveExtensibility() = %q, want %q", got, tt.want)
			}
		})
	}
}
