package handler

import (
	"bytes"
	"encoding/hex"
	"io"

	"github.com/TAAL-GmbH/mapi/client"
	"github.com/TAAL-GmbH/mapi/config"
	"github.com/TAAL-GmbH/mapi/models"
	"github.com/labstack/echo/v4"
)

// NewLogger returns a Logger middleware with config.
func NewLogger(mapiClient client.Interface, config *config.AppConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			req := c.Request()

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
			accessLog.IP = c.RealIP()
			accessLog.Host = req.Host
			accessLog.RequestURI = req.RequestURI
			accessLog.Method = req.Method
			accessLog.Referer = req.Referer()
			accessLog.UserAgent = req.UserAgent()
			accessLog.RequestData = hex.EncodeToString(b)

			err = accessLog.Save(req.Context())
			if err = next(c); err != nil {
				c.Error(err)
			}

			c.Set("access_log", &accessLog)

			return
		}
	}
}
