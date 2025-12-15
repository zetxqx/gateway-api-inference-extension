package main

import (
	"fmt"
	"strings"
)

type Tokenizer struct {
	vocab map[uint32]string
}

func NewTokenizer() *Tokenizer {
	return &Tokenizer{
		vocab: map[uint32]string{
			1:  "Hello",
			2:  ",",
			3:  " ",
			4:  "world",
			5:  "!",
			10: "Tell",
			11: " ",
			12: "me",
			13: " ",
			14: "a",
			15: " ",
			16: "story",
			17: ".",
		},
	}
}

func (t *Tokenizer) Detokenize(ids []uint32) string {
	var sb strings.Builder
	for _, id := range ids {
		if val, ok := t.vocab[id]; ok {
			sb.WriteString(val)
		} else {
			// Fallback for unknown tokens
			sb.WriteString(fmt.Sprintf("<%d>", id))
		}
	}
	return sb.String()
}
