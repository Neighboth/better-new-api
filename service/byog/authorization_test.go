package byog

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestByogPriceBoundsAndRounding(t *testing.T) {
	for _, price := range []byogPrice{{value: 0.3}, {fixed: true, value: 123.4}, {value: 0}} {
		reserve, err := price.quota(100, 200, true)
		require.NoError(t, err)
		for _, usage := range [][2]int{{0, 0}, {1, 1}, {100, 200}, {50, 20}} {
			actual, err := price.quota(usage[0], usage[1], false)
			require.NoError(t, err)
			require.LessOrEqual(t, actual, reserve)
		}
	}
	_, err := (byogPrice{value: math.Inf(1)}).quota(1, 1, true)
	require.Error(t, err)
	_, err = (byogPrice{value: float64(common.MaxQuota)}).quota(1, 1, true)
	require.Error(t, err)
	_, err = (byogPrice{value: 1}).quota(-1, 1, true)
	require.Error(t, err)
}
