package server

import "github.com/golang-jwt/jwt"

type JWTCustomClaims struct {
	ClientID string `json:"client_id"`
	Name     string `json:"name"`
	Admin    bool   `json:"admin"`
	jwt.StandardClaims
}
