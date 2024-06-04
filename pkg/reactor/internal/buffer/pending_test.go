package buffer

import (
	"reflect"
	"testing"
)

func TestPendingBuffers(t *testing.T) {
	tests := []struct {
		in      []byte
		consume int
		want    []byte
	}{
		{
			in:      []byte("foo"),
			consume: 0,
			want:    []byte("foo"),
		},
		{
			in:      []byte("foo"),
			consume: 2,
			want:    []byte("o"),
		},
		{
			in:      []byte("foo"),
			consume: 3,
			want:    []byte{},
		},
		{
			in:      []byte("bar"),
			consume: 1,
			want:    []byte("ar"),
		},
		{
			in:      []byte("bar"),
			consume: 4,
			want:    []byte{},
		},
		{
			in:      []byte("bar"),
			consume: -1,
			want:    []byte("bar"),
		},
		{
			in:      nil,
			consume: 1,
			want:    nil,
		},
	}
	for i, tt := range tests {
		in := tt.in

		pb := NewPendingBuffer(in)

		pb.Consume(tt.consume)

		if !reflect.DeepEqual(pb.Bytes(), tt.want) {
			t.Errorf("%d. after consume(%d) = %v, want %v", i, tt.consume, in, tt.want)
		}
	}
}
