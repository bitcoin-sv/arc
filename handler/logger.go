package handler

import (
	"bytes"
	"io"

	"github.com/TAAL-GmbH/mapi"
	"github.com/TAAL-GmbH/mapi/client"
	"github.com/TAAL-GmbH/mapi/config"
	"github.com/TAAL-GmbH/mapi/models"
	"github.com/labstack/echo/v4"
)

// NewLogger returns a Logger middleware with config.
func NewLogger(mapiClient client.Interface, config *config.AppConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) (err error) {
			req := ctx.Request()

			var b []byte
			if config.LogRequestBody {
				// read the body for logging and re-set for next handler
				if req.Body != nil {
					defer func() {
						_ = req.Body.Close()
					}()
					b, err = io.ReadAll(req.Body)
					if err != nil {
						return
					}
					req.Body = io.NopCloser(bytes.NewReader(b))
				}
			}

			accessLog := models.NewLogAccess(models.WithClient(mapiClient), models.New())
			accessLog.IP = ctx.RealIP()
			accessLog.Host = req.Host
			accessLog.RequestURI = req.RequestURI
			accessLog.Method = req.Method
			accessLog.Referer = req.Referer()
			accessLog.UserAgent = req.UserAgent()
			accessLog.RequestData = string(b)

			var user *mapi.User
			if user, err = GetEchoUser(ctx, config.Security); err != nil {
				return err
			}
			if user != nil {
				accessLog.ClientID = user.ClientID
			}

			if err = accessLog.Save(req.Context()); err != nil {
				return err
			}

			ctx.Set("access_log", accessLog)

			if err = next(ctx); err != nil {
				ctx.Error(err)
			}

			return
		}
	}
}
