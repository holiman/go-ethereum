// Copyright 2021 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package rawdb

import (
	"bytes"
	"io"

	"github.com/golang/snappy"
)

// BufferedSnapWriter writes snappy in block format, and can be reused. It is
// reset when WriteTo is called.
type BufferedSnapWriter struct {
	buf bytes.Buffer
	dst []byte
}

// Write appends the contents of p to the buffer.
func (s *BufferedSnapWriter) Write(p []byte) (n int, err error) {
	return s.buf.Write(p)
}

// WriteTo snappy-compresses the data, writes to the given writer and truncates
// instantiated buffers.
func (s *BufferedSnapWriter) WriteTo(w io.Writer) {
	s.dst = snappy.Encode(s.dst, s.buf.Bytes())
	w.Write(s.dst)
	s.dst = s.dst[:0]
	s.buf.Reset()
}

// WriteDirectTo snappy-compresses the data, writes to the given writer.
// This method writes _only_ the input 'buf'.
func (s *BufferedSnapWriter) WriteDirectTo(w io.Writer, buf []byte) {
	s.dst = snappy.Encode(s.dst, buf)
	w.Write(s.dst)
	s.dst = s.dst[:0]
}
