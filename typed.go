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
