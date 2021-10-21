package io

import (
	"bytes"
	"io"
)

var lineBufInitialCapacity = 140
var newline byte = 10

// PrefixWriter is a writer that assumes to receive text input and
// prefixes every line with a given prefix string.
type PrefixWriter struct {
	Underlying       io.Writer
	Prefix           string
	lineBuffer       *bytes.Buffer
	prefixOnNextChar bool
}

// NewPrefixWriter creates a new PrefixWriter. This is the only way
// to create a PrefixWriter.
func NewPrefixWriter(underlying io.Writer, prefix string) PrefixWriter {
	pw := PrefixWriter{
		Underlying:       underlying,
		Prefix:           prefix,
		lineBuffer:       bytes.NewBuffer(make([]byte, 0, lineBufInitialCapacity)),
		prefixOnNextChar: true,
	}
	return pw
}

func (w PrefixWriter) Write(inputBytes []byte) (int, error) {
	nPrefixes := 0

	if w.lineBuffer.Len() != 0 {
		// Buffer is filled with initial prefix and writer has never
		// written.
		nPrefixes++
	}

	for _, b := range inputBytes {
		if w.prefixOnNextChar {
			if _, err := w.lineBuffer.WriteString(w.Prefix); err != nil {
				return 0, err
			}
			nPrefixes++
		}
		if err := w.lineBuffer.WriteByte(b); err != nil {
			return 0, err
		}
		w.prefixOnNextChar = b == newline
	}

	bufBytes := w.lineBuffer.Bytes()
	bytesN, err := w.Underlying.Write(bufBytes)
	if err != nil {
		return 0, err
	}
	n := bytesN - nPrefixes*len(w.Prefix)
	w.lineBuffer.Reset()
	return n, nil
}
