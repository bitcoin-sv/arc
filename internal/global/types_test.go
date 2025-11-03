package global_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/bitcoin-sv/arc/internal/global"
	"github.com/bitcoin-sv/arc/internal/global/mocks"
)

func TestStoppablesShutdown(t *testing.T) {
	t.Run("shutdown stoppable", func(_ *testing.T) {
		// given
		stoppables := global.Stoppables{&mocks.StoppableMock{ShutdownFunc: func() {}}, &mocks.StoppableMock{ShutdownFunc: func() {}}}

		// when
		stoppables.Shutdown()
	})
}

func TestStoppablesWithErrorShutdown(t *testing.T) {
	t.Run("shutdown stoppable", func(_ *testing.T) {
		// given
		stoppables := global.StoppablesWithError{
			&mocks.StoppableWithErrorMock{ShutdownFunc: func() error { return nil }},
			&mocks.StoppableWithErrorMock{ShutdownFunc: func() error { return errors.New("some error") }},
		}

		// when
		stoppables.Shutdown(slog.Default())
	})
}

func TestStoppablesWithContextShutdown(t *testing.T) {
	t.Run("shutdown stoppable", func(_ *testing.T) {
		// given
		stoppables := global.StoppablesWithContext{
			&mocks.StoppableWithContextMock{ShutdownFunc: func(_ context.Context) error { return nil }},
			&mocks.StoppableWithContextMock{ShutdownFunc: func(_ context.Context) error { return errors.New("some error") }},
		}

		// when
		stoppables.Shutdown(context.TODO(), slog.Default())
	})
}
