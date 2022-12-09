package handler

import (
	arc "github.com/TAAL-GmbH/arc"
	"github.com/TAAL-GmbH/arc/config"
	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
)

func GetEchoUser(ctx echo.Context, securityConfig *config.SecurityConfig) (*arc.User, error) {

	if securityConfig != nil {
		if securityConfig.Type == config.SecurityTypeJWT {
			user := ctx.Get("user").(*jwt.Token)
			claims := user.Claims.(*arc.JWTCustomClaims)

			return &arc.User{
				ClientID: claims.ClientID,
				Name:     claims.Name,
				Admin:    claims.Admin,
			}, nil
		} else if securityConfig.Type == config.SecurityTypeCustom {
			if securityConfig.CustomGetUser != nil {
				return securityConfig.CustomGetUser(ctx)
			}
		}
	}

	return &arc.User{}, nil
}
