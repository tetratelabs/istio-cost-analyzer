// Copyright 2022 Tetrate
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
