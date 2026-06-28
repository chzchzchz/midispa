package theory

import (
	"math"
)

func CentsHz(fromHz, toHz float64) float64 {
	return 1200.0 * math.Log2(toHz/fromHz)
}
