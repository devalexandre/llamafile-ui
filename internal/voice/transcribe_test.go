package voice

import "testing"

func TestCleanText(t *testing.T) {
	cases := map[string]string{
		"[Music]\n":                          "",
		" [BLANK_AUDIO]\n (risos)\n":         "",
		" Olá, tudo bem?\n [Music]\n Sim.\n": "Olá, tudo bem? Sim.",
		" O preço é [R$ 10] hoje\n":          "O preço é [R$ 10] hoje",
	}
	for in, want := range cases {
		if got := cleanText(in); got != want {
			t.Errorf("cleanText(%q) = %q, want %q", in, got, want)
		}
	}
}
