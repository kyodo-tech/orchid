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
