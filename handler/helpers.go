package handler

import (
	"github.com/TAAL-GmbH/mapi"
	"github.com/TAAL-GmbH/mapi/config"
	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
)

func GetEchoUser(ctx echo.Context, securityConfig *config.SecurityConfig) (*mapi.User, error) {

	if securityConfig != nil {
		if securityConfig.Type == config.SecurityTypeJWT {
			user := ctx.Get("user").(*jwt.Token)
			claims := user.Claims.(*mapi.JWTCustomClaims)

			return &mapi.User{
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

	return &mapi.User{}, nil
}
