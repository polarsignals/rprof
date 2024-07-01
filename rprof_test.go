package rprof

import (
	"fmt"
	"testing"
)

func TestClosestPowerOfTwo(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input         int
		expectedPower uint8
	}{{
		input:         1,
		expectedPower: 0,
	}, {
		input:         2,
		expectedPower: 1,
	}, {
		input:         4,
		expectedPower: 2,
	}, {
		input:         14,
		expectedPower: 4,
	}, {
		input:         31,
		expectedPower: 5,
	}}

	for _, c := range cases {
		c := c
		t.Run(fmt.Sprintf("%d", c.input), func(t *testing.T) {
			res := nextPowerOfTwo(c.input)
			if res != c.expectedPower {
				t.Fatalf("expected %d but got %d", c.expectedPower, res)
			}
		})
	}
}
