package handler

import (
	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
	"github.com/taal/mapi/config"
	"github.com/taal/mapi/server"
)

type User struct {
	ClientID string `json:"client_id"`
	Name     string `json:"name"`
	Admin    bool   `json:"admin"`
}

func GetEchoUser(ctx echo.Context, securityConfig *config.SecurityConfig) *User {

	if securityConfig != nil {
		if securityConfig.Provider == config.SecurityTypeBearerAuth {
			user := ctx.Get("user").(*jwt.Token)
			claims := user.Claims.(*server.JWTCustomClaims)

			return &User{
				ClientID: claims.ClientID,
				Name:     claims.Name,
				Admin:    claims.Admin,
			}
		} else if securityConfig.Provider == config.SecurityTypeApiKey {
			// the api_key_user should have been set by the api key handler
			if user, ok := ctx.Get("api_key_user").(*User); ok {
				return user
			}
		}
	}

	return &User{}
}
