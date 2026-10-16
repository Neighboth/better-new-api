package hfendpoints

import "github.com/QuantumNous/new-api/common"

func marshal(v any) ([]byte, error) {
	return common.Marshal(v)
}

func unmarshal(data []byte, v any) error {
	return common.Unmarshal(data, v)
}
