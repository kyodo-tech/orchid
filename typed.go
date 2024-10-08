// Copyright 2024 Kyodo Tech合同会社
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package orchid

import (
	"context"
	"encoding/json"
)

func TypedActivity[T any](activity func(ctx context.Context, input T) (T, error)) Activity {
	return func(ctx context.Context, input []byte) ([]byte, error) {
		var in T
		if len(input) > 0 {
			if err := json.Unmarshal(input, &in); err != nil {
				return nil, err
			}
		}
		out, outErr := activity(ctx, in)
		outData, _ := json.Marshal(out)
		return outData, outErr
	}
}
