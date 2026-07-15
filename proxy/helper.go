package proxy

import (
	"context"
	"errors"
	"net"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/fmotalleb/junction/utils"
)

// relayTraffic concurrently relays data between two network connections in both directions until either connection is closed or an error occurs.
// Logs connection closure and errors for diagnostic purposes.
func relayTraffic(ctx context.Context, src, dst net.Conn, logger *zap.Logger) {
	defer func() {
		_ = src.Close()
		_ = dst.Close()
	}()
	errs, _ := errgroup.WithContext(ctx)
	errs.Go(
		func() error {
			if err := utils.Copy(dst, src); err != nil {
				return errors.Join(errors.New("failed to write to dst"), err)
			}
			return nil
		},
	)
	errs.Go(
		func() error {
			if err := utils.Copy(src, dst); err != nil {
				return errors.Join(errors.New("failed to write to dst"), err)
			}
			return nil
		},
	)
	// Wait for the first error or closure
	err := errs.Wait()
	if err != nil {
		if errors.Is(err, net.ErrClosed) {
			logger.Debug("connection closed (normal)", zap.Error(err))
		} else {
			logger.Warn("connection collapsed", zap.Error(err))
		}
	}
}
