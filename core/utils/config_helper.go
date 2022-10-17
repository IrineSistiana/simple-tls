package utils

import (
	"golang.org/x/exp/constraints"
)

func SetDefaultNum[K constraints.Integer | constraints.Float](p *K, d K) {
	if *p == 0 {
		*p = d
	}
}
