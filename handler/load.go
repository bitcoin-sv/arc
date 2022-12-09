package handler

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"

	arc "github.com/TAAL-GmbH/arc"
	"github.com/TAAL-GmbH/arc/client"
	"github.com/TAAL-GmbH/arc/config"
	"github.com/TAAL-GmbH/arc/dictionary"
	"github.com/TAAL-GmbH/arc/models"
	"github.com/bitcoinsv/bsvd/chaincfg/chainhash"
	"github.com/deepmap/oapi-codegen/pkg/middleware"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/libsv/go-bk/bec"
	"github.com/libsv/go-bk/wif"
	"github.com/mrz1836/go-logger"
)

func LoadArcHandler(e *echo.Echo, appConfig *config.AppConfig) error {

	// check the swagger definition against our requests
	CheckSwagger(e)

	// Check the security requirements
	CheckSecurity(e, appConfig)

	// Create an instance of our handler which satisfies the generated interface
	var opts []client.Options
	opts = append(opts, client.WithMinerID(appConfig.MinerID))
	opts = append(opts, client.WithMigrateModels(models.BaseModels))

	// Add cachestore
	if appConfig.Cachestore.Engine == "redis" {
		opts = append(opts, client.WithRedis(appConfig.Redis))
	} else {
		// setup an in memory local cache, using free cache
		opts = append(opts, client.WithFreeCache())
	}

	// Add datastore
	// by default an SQLite database will be used in ./arc.db if no datastore options are set in config
	if appConfig.Datastore != nil {
		opts = append(opts, client.WithDatastore(appConfig.Datastore))
	}

	// set the nodes config, if set
	if appConfig.Nodes != nil {
		opts = append(opts, client.WithNodeConfig(appConfig.Nodes))
	}

	if len(appConfig.Metamorph) > 0 {
		opts = append(opts, client.WithMetamorphs(appConfig.Metamorph))
	}

	// load the api, using the default handler
	c, err := client.New(opts...)
	if err != nil {
		return err
	}

	// run the custom migrations for all active models
	activeModels := c.Models()
	if len(activeModels) > 0 {
		// will run the custom model Migrate() method for all models
		for _, model := range activeModels {
			model.(models.ModelInterface).SetOptions(models.WithClient(c))
			if err = model.(models.ModelInterface).Migrate(c.Datastore()); err != nil {
				return err
			}
		}
	}

	var api arc.HandlerInterface
	if api, err = NewDefault(c, WithSecurityConfig(appConfig.Security)); err != nil {
		return err
	}

	// Register the ARC API
	arc.RegisterHandlers(e, api)

	// e.Use(signBody(appConfig))

	TEMPTEMPGenerateTestToken(appConfig)

	return nil
}

func TEMPTEMPGenerateTestToken(appConfig *config.AppConfig) {
	// TEMP TEMP TEMP
	claims := &arc.JWTCustomClaims{
		ClientID:       "test",
		Name:           "Test User",
		Admin:          false,
		StandardClaims: jwt.StandardClaims{},
	}

	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Generate encoded token and send it as response.
	var signedToken string
	signedToken, err := token.SignedString([]byte(appConfig.Security.BearerKey))
	if err != nil {
		panic(err)
	}
	fmt.Println(signedToken)
}

type bodyResponseWriter struct {
	io.Writer
	http.ResponseWriter
	buffer    *bytes.Buffer
	wroteBody bool
}

func (w *bodyResponseWriter) WriteHeader(code int) {
	w.ResponseWriter.WriteHeader(code)
}

func (w *bodyResponseWriter) Write(b []byte) (int, error) {
	w.wroteBody = true
	return w.Writer.Write(b)
}

func (w *bodyResponseWriter) Flush() {
	// noop
}

func (w *bodyResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.ResponseWriter.(http.Hijacker).Hijack()
}

func bufferPool() sync.Pool {
	return sync.Pool{
		New: func() interface{} {
			b := &bytes.Buffer{}
			return b
		},
	}
}

// signBody buffers the body and signs it with the minerID
// TODO this is not working yet - need to figure it out
func signBody(config *config.AppConfig) echo.MiddlewareFunc {
	bpool := bufferPool()
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.MinerID != nil {
				res := c.Response()
				buf := bpool.Get().(*bytes.Buffer)
				buf.Reset()

				// create a buffer writer
				writer := &bodyResponseWriter{Writer: buf, ResponseWriter: res.Writer, buffer: buf}
				res.Writer = writer

				res.After(func() {
					privateKey, err := PrivateKeyFromString(config.MinerID.PrivateKey)
					messageHash := chainhash.HashB(writer.buffer.Bytes())
					var sigBytes []byte
					if sigBytes, err = bec.SignCompact(bec.S256(), privateKey, messageHash, true); err != nil {
						fmt.Printf("ERROR: %s", err.Error())
					}
					base64Sig := base64.StdEncoding.EncodeToString(sigBytes)
					fmt.Printf("Signature: %s", base64Sig)
					res.Header().Add("Signature", base64Sig)

					// flush the buffered response
					_, _ = writer.ResponseWriter.Write(writer.buffer.Bytes())
					if flusher, ok := writer.ResponseWriter.(http.Flusher); ok {
						flusher.Flush()
					}
				})
			}

			return next(c)
		}
	}
}

// PrivateKeyFromString turns a private key (hex encoded string) into a bec.PrivateKey
// from github.com/BitcoinSchema/go-bitcoin
func PrivateKeyFromString(privateKey string) (*bec.PrivateKey, error) {

	decodedWif, err := wif.DecodeWIF(privateKey)
	if err != nil {
		return nil, err
	}

	return decodedWif.PrivKey, nil
}

// CheckSwagger validates the request against the swagger definition
func CheckSwagger(e *echo.Echo) *openapi3.T {

	swagger, err := arc.GetSwagger()
	if err != nil {
		logger.Fatalf(dictionary.GetInternalMessage(dictionary.ErrorLoadingSwaggerSpec), err.Error())
		os.Exit(1)
	}

	// Clear out the servers array in the swagger spec, that skips validating
	// that server names match. We don't know how this thing will be run.
	swagger.Servers = nil
	// Clear out the security requirements, we check this ourselves
	swagger.Security = nil

	// Use our validation middleware to check all requests against the OpenAPI schema.
	e.Use(middleware.OapiRequestValidator(swagger))

	return swagger
}

// CheckSecurity validates a request against the configured security
func CheckSecurity(e *echo.Echo, appConfig *config.AppConfig) {

	if appConfig.Security != nil {
		if appConfig.Security.Type == config.SecurityTypeJWT {
			e.Pre(echomiddleware.JWTWithConfig(echomiddleware.JWTConfig{
				SigningKey: []byte(appConfig.Security.BearerKey),
				Claims:     &arc.JWTCustomClaims{},
			}))
		} else if appConfig.Security.Type == config.SecurityTypeCustom {
			e.Pre(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(ctx echo.Context) error {
					_, err := appConfig.Security.CustomGetUser(ctx)
					if err != nil {
						ctx.Error(err)
					}
					return next(ctx)
				}
			})
		} else {
			panic(fmt.Errorf("unknown security type: %s", appConfig.Security.Type))
		}
	}
}
