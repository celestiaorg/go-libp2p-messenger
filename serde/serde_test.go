package serde

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalUnmarshal(t *testing.T) {
	in := &fakeMsg{data: []byte("test")}
	buf := make([]byte, 100)

	n, err := Marshal(in, buf)
	require.Nil(t, err)
	assert.Greater(t, n, in.Size())

	out := &fakeMsg{}
	nn, err := Unmarshal(out, buf)
	require.Nil(t, err)
	assert.Equal(t, n, nn)

	assert.Equal(t, in, out)
}

func TestWriteRead(t *testing.T) {
	in := &fakeMsg{data: []byte("test")}
	rw := &testRW{}

	n, err := Write(rw, in)
	require.Nil(t, err)
	assert.NotZero(t, n)
	assert.Equal(t, n, rw.w)
	assert.NotEqual(t, n, in.Size())

	out := &fakeMsg{}
	nn, err := Read(rw, out)
	require.Nil(t, err)
	assert.NotZero(t, nn)
	assert.Equal(t, nn, rw.r)
	assert.Equal(t, n, nn)
	assert.Equal(t, in, out)
}

type testRW struct {
	buf  []byte
	r, w int // read, written
}

func (rw *testRW) Write(b []byte) (n int, err error) {
	if len(rw.buf) == rw.w {
		rw.buf = append(rw.buf, make([]byte, len(b))...)
	}
	n = copy(rw.buf[rw.w:], b)
	rw.w += n
	return
}

func (rw *testRW) Read(b []byte) (n int, err error) {
	if len(rw.buf) == rw.r {
		return 0, io.EOF
	}
	n = copy(b, rw.buf[rw.r:])
	rw.r += n
	return
}

type fakeMsg struct {
	data []byte
}

func (f *fakeMsg) Size() int {
	return len(f.data)
}

func (f *fakeMsg) MarshalTo(buf []byte) (int, error) {
	return copy(buf, f.data), nil
}

func (f *fakeMsg) Unmarshal(data []byte) error {
	f.data = make([]byte, len(data))
	copy(f.data, data)
	return nil
}
