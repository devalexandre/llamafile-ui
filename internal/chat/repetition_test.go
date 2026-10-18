package chat

import (
	"strings"
	"testing"
)

func TestRepetitionCut(t *testing.T) {
	loop := "A quantização reduz bits.\n\n" +
		"**Representação de Bits:** O valor numérico é representado em bits, onde cada bit representa um valor finito.\n\n" +
		"Divisão de Bits: a divisão de um valor numérico em partes finitas, geralmente de 2 bits.\n\n" +
		"Representação de Bits: o valor numérico é representado em bits, onde cada bit representa um valor finito.\n\n" +
		"Divisão de Bits: A divisão de um valor numérico em partes finitas, geralmente de 2 bits.\n\n" +
		"- Representação de bits: O valor numérico é representado em bits, onde cada bit representa um valor finito.\n"
	cut, ok := repetitionCut(loop)
	if !ok {
		t.Fatal("loop not detected")
	}
	if strings.Count(normalizeParagraph(loop[:cut]), "representaçãodebits") != 2 {
		t.Fatalf("should keep two occurrences: %q", loop[:cut])
	}
	normal := "Passo 1: instale o app no computador.\nPasso 2: baixe um modelo recomendado.\nPasso 3: comece a conversar no chat.\n"
	if _, ok := repetitionCut(normal); ok {
		t.Fatal("false positive")
	}
}
