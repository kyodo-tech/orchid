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

package middleware

import (
	"context"
	"errors"

	"github.com/kyodo-tech/orchid"
)

var _ orchid.Middleware = Logging

func Logging(activity orchid.Activity) orchid.Activity {
	return func(ctx context.Context, input []byte) ([]byte, error) {
		logger := orchid.Logger(ctx)
		name := orchid.ActivityName(ctx)

		logger.Info("Starting activity", "activity", name)
		output, outErr := activity(ctx, input)

		var dynamicRoute *orchid.DynamicRoute
		dynamicRouteErr := errors.As(outErr, &dynamicRoute)

		if outErr != nil && !dynamicRouteErr || dynamicRouteErr && dynamicRoute.Err != nil {
			logger.Error("Activity failed", "activity", name, "error", outErr)
		} else if dynamicRouteErr {
			logger.Info("Activity completed with route", "activity", name, "route", dynamicRoute.Key)
		} else {
			logger.Info("Activity completed", "activity", name)
		}
		return output, outErr
	}
}
