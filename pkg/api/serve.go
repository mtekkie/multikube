package api

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/amimof/multikube/pkg/config"
	"github.com/amimof/multikube/pkg/middleware"
	"github.com/pkg/errors"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

var (
	ErrNotImplemented 	= errors.New("The method is not allowed for this service.")
	ErrUnMarshalFailed 	= errors.New("The submitted data was unprocessable, check syntax")
	ErrInternalServer 	= errors.New("Internal error")
	ErrServiceNotFound 	= errors.New("The requested service can not be found")
	ErrObjectDoesNotExist = errors.New("The requested object does not exist")

)

type MkApi struct {

	apis map[string]ApiInterface
	Config *config.Config


}

type ApiInterface interface {

	Create ([]byte) (interface{}, error)
 	Read (string) (interface{}, error)
 	//Update (string, []byte) ()
 	//Delete ( string ) ()
}


// NewApi creates a new Api
func NewApi() *MkApi {
	return &MkApi{
		apis: make (map[string]ApiInterface),
		Config: &config.Config{
			OIDCIssuerURL:     "",
			OIDCPollInterval:  time.Second * 2,
			OIDCUsernameClaim: "sub",
			RS256PublicKey:    &x509.Certificate{},
			JWKS:              &config.JWKS{},
		},
	}
}


// Use chains all middlewares and applies a context to the request flow
func (a *MkApi) Use(mw ...middleware.Middleware) middleware.Middleware {
	return func(c *config.Config, final http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			last := final
			for i := len(mw) - 1; i >= 0; i-- {
				last = mw[i](a.Config, last)
			}
			last.ServeHTTP(w, r)
		})
	}
}

// ServeHTTP serves the API.
func (a *MkApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	path := strings.Split(r.RequestURI, "/")

	// Verify api exists
	if a.apis[path[1]] == nil {
		handleError(w,r,ErrServiceNotFound)
		return
	}


	switch r.Method {

	case "GET":

		result, err  := a.apis[path[1]].Read(strings.Join(path[2:], "/"))
		if err != nil {
			handleError(w,r,ErrInternalServer)
			return
		}

		payload, err := json.Marshal(result)
		if err != nil {
			handleError(w,r,err)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		w.Write(payload)

		break

	case "POST":
		b, err := ioutil.ReadAll(r.Body)

		if err != nil {
			handleError(w,r, ErrInternalServer)
			return
		}

		result, err := a.apis[path[1]].Create(b)

		if err != nil {
			handleError(w,r, err)
			return
		}

		payload, err := json.Marshal(result)
		if err != nil {
			handleError(w,r,ErrInternalServer)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.Write(payload)

		break

	case "DELETE":

	default:
		handleError(w,r, ErrNotImplemented)
	}




}

func (a *MkApi) Register (name string, apiInterface ApiInterface) {

	a.apis[name] = apiInterface
	log.Printf("Registered API %s", name)

}

func handleError(w http.ResponseWriter, r *http.Request, err error ){

	switch err {

	case ErrNotImplemented:
		w.WriteHeader(http.StatusMethodNotAllowed)
		msg := fmt.Sprintf("%s.  [%s]%s", err , r.Method, r.RequestURI )
		w.Write([]byte (msg))
		break

	case ErrServiceNotFound:
		w.WriteHeader(http.StatusNotFound)
		msg := fmt.Sprintf("[%s] %s", r.RequestURI,err  )
		w.Write([]byte (msg))
		break

		default:
		w.WriteHeader(http.StatusInternalServerError)
		msg := fmt.Sprintf("Error processing request: %s", err )
		w.Write([]byte (msg))
	}

}