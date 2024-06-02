package porcupine

import (
	"math/bits"
	"testing"
)

func TestTest(t *testing.T) {
	a := uint64(0xFF)
	t.Errorf("%d\n", bits.OnesCount64(a))
}
