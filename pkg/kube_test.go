package pkg

import (
	"math"
	"reflect"
	"testing"
)

func TestKubeClient_CollapseLocalityCalls(t *testing.T) {
	k := &KubeClient{}
	tests := []struct {
		name     string
		calls    []*Call
		expected []*Call
	}{
		{
			name:     "no calls",
			calls:    []*Call{},
			expected: []*Call{},
		},
		{
			name: "collapse calls",
			calls: []*Call{
				{
					From:     "us-west1-b",
					To:       "us-east1-b",
					CallSize: uint64(math.Pow(10, 9)),
				},
				{
					From:     "us-west1-b",
					To:       "us-east1-b",
					CallSize: 2 * uint64(math.Pow(10, 9)),
				},
				{
					From:     "us-west1-a",
					To:       "us-east1-a",
					CallSize: uint64(math.Pow(10, 9)),
				},
			},
			expected: []*Call{
				{
					From:     "us-west1-b",
					To:       "us-east1-b",
					CallSize: 3 * uint64(math.Pow(10, 9)),
				},
				{
					From:     "us-west1-a",
					To:       "us-east1-a",
					CallSize: uint64(math.Pow(10, 9)),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := k.CollapseLocalityCalls(tt.calls); !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("KubeClient.CollapseLocalityCalls() = %v, want %v", got, tt.expected)
			}
		})
	}
}
