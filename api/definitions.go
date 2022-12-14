package api

import "github.com/golang-jwt/jwt"

const (
	MetadataField = "metadata"
)

type JWTCustomClaims struct {
	ClientID string `json:"client_id"`
	Name     string `json:"name"`
	Admin    bool   `json:"admin"`
	jwt.StandardClaims
}

type User struct {
	ClientID string `json:"client_id"`
	Name     string `json:"name"`
	Admin    bool   `json:"admin"`
}
