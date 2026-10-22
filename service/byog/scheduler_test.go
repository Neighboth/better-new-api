package byog

import (
	"testing"

	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
)

func TestAvailableRamMB(t *testing.T) {
	cases := []struct {
		name      string
		physical  int
		allocated int
		want      int
	}{
		{
			name:      "the whole memory of the device is used when nothing is given",
			physical:  8192,
			allocated: 0,
			want:      8192,
		},
		{
			name:      "the memory that the user gives limits the models",
			physical:  8192,
			allocated: 3,
			want:      3072,
		},
		{
			name:      "the memory of the device is smaller than the memory that is given",
			physical:  2048,
			allocated: 8,
			want:      2048,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			node := &model.ByogNode{
				PhysicalRamMB:  testCase.physical,
				AllocatedRamGB: testCase.allocated,
			}
			assert.Equal(t, testCase.want, availableRamMB(node))
		})
	}
}

func TestAvailableRamMBWithoutNode(t *testing.T) {
	assert.Equal(t, 0, availableRamMB(nil))
}
