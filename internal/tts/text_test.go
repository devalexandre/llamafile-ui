package tts

import "testing"

func TestClean(t *testing.T) {
	md := "# Título\n\nUse **negrito** e `go run` [aqui](https://x.y) [1].\n\n```go\nfmt.Println()\n```\n\n- item um\n- item dois\n\n| a | b |\n|---|---|\n| 1 | 2 |"
	want := "Título\nUse negrito e go run aqui.\n(trecho de código)\nitem um\nitem dois\na, b\n1, 2"
	if got := Clean(md); got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}
