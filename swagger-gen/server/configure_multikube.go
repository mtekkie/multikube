// This file is safe to edit. Once it exists it will not be overwritten

package server

import (
	"crypto/tls"
	"net/http"
	//"log"
	"os"
	//"io/ioutil"

	errors "github.com/go-openapi/errors"
	runtime "github.com/go-openapi/runtime"
	middleware "github.com/go-openapi/runtime/middleware"
	graceful "github.com/tylerb/graceful"

	"gitlab.com/amimof/multikube/api/v1/server/restapi"
	"gitlab.com/amimof/multikube/api/v1/server/restapi/clusters"
	
	//"github.com/go-openapi/swag"
)

//go:generate swagger generate server --target ../api/v1 --name multikube --spec ../api/v1/swagger.yml --api-package restapi --server-package server
func configureFlags(api *restapi.MultikubeAPI) {
}

func configureAPI(api *restapi.MultikubeAPI) http.Handler {
	// configure the api here
	api.ServeError = errors.ServeError

	// Set your custom logger if needed. Default one is log.Printf
	// Expected interface func(string, ...interface{})
	//
	// Example:
	// api.Logger = log.Printf

	api.JSONConsumer = runtime.JSONConsumer()

	api.JSONProducer = runtime.JSONProducer()

	api.ClustersGetClustersHandler = clusters.GetClustersHandlerFunc(func(params clusters.GetClustersParams) middleware.Responder {
		return middleware.NotImplemented("operation clusters.GetClusters has not yet been implemented")
	})
	api.ClustersAddOneHandler = clusters.AddOneHandlerFunc(func(params clusters.AddOneParams) middleware.Responder {
		return middleware.NotImplemented("operation clusters.AddOne has not yet been implemented")
	})
	api.ClustersDestroyOneHandler = clusters.DestroyOneHandlerFunc(func(params clusters.DestroyOneParams) middleware.Responder {
		return middleware.NotImplemented("operation clusters.DestroyOne has not yet been implemented")
	})
	api.ClustersGetOneHandler = clusters.GetOneHandlerFunc(func(params clusters.GetOneParams) middleware.Responder {
		return middleware.NotImplemented("operation clusters.GetOne has not yet been implemented")
	})
	api.ClustersUpdateOneHandler = clusters.UpdateOneHandlerFunc(func(params clusters.UpdateOneParams) middleware.Responder {
		return middleware.NotImplemented("operation clusters.UpdateOne has not yet been implemented")
	})

	api.ServerShutdown = func() {}

	return setupGlobalMiddleware(api.Serve(setupMiddlewares))
}

// The TLS configuration before HTTPS server starts.
func configureTLS(tlsConfig *tls.Config) {
	// Make all necessary changes to the TLS configuration here.
}

// As soon as server is initialized but not run yet, this function will be called.
// If you need to modify a config, store server instance to stop it individually later, this is the place.
// This function can be called multiple times, depending on the number of serving schemes.
// scheme value will be set accordingly: "http", "https" or "unix"
func configureServer(s *graceful.Server, scheme, addr string) {
	setupConfig()
}

// The middleware configuration is for the handler executors. These do not apply to the swagger.json document.
// The middleware executes after routing but before authentication, binding and validation
func setupMiddlewares(handler http.Handler) http.Handler {
	return handler
}

// The middleware configuration happens before anything, this middleware also applies to serving the swagger.json document.
// So this is a good place to plug in a panic handling middleware, logging and metrics
func setupGlobalMiddleware(handler http.Handler) http.Handler {
	return handler
}
