package server

import (
	"net/http"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
)

//HandleError wraps the HTTP Handler so that errors can be handled and non-200 response codes issued.
func HandleError(s *client.Schemas, t func(http.ResponseWriter, *http.Request) (int, error)) http.Handler {
	return api.ApiHandler(s, http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if code, err := t(rw, req); err != nil {
			logrus.Errorf("Error in request, code: %d: %s", code, err)
			apiContext := api.GetApiContext(req)
			rw.WriteHeader(code)

			apiContext.Write(&errObj{
				Resource: client.Resource{
					Type: "error",
				},
				Status:  strconv.Itoa(code),
				Message: err.Error(),
			})
		} else {
			if code != 200 {
				rw.WriteHeader(code)
			}
		}
	}))
}

// NewRouter creates and adds all the Routes for a Rancher API and Token service
func NewRouter() *mux.Router {
	schemas := &client.Schemas{}
	f := HandleError

	schemas.AddType("apiVersion", client.Resource{})
	schemas.AddType("schema", client.Schema{})

	schemas.AddType("vaultTokenInput", VaultTokenInput{})
	schemas.AddType("vaultIntermediateToken", VaultIntermediateTokenResponse{})

	err := schemas.AddType("error", errObj{})
	err.CollectionMethods = []string{}

	router := mux.NewRouter().StrictSlash(false)

	// Rancher API Routes
	router.Methods("GET").Path("/v1-vault-driver").Handler(api.VersionHandler(schemas, "v1-vault-driver"))
	router.Methods("GET").Path("/v1-vault-driver/").Handler(api.VersionHandler(schemas, "v1-vault-driver"))

	router.Methods("GET").Path("/v1-vault-driver/schemas").Handler(api.SchemasHandler(schemas))
	router.Methods("GET").Path("/v1-vault-driver/schemas/").Handler(api.SchemasHandler(schemas))

	router.Methods("GET").Path("/v1-vault-driver/schemas/{id}").Handler(api.SchemaHandler(schemas))
	router.Methods("GET").Path("/v1-vault-driver/schemas/{id}/").Handler(api.SchemaHandler(schemas))

	// Application Routes
	router.Methods("POST").Path("/v1-vault-driver/tokens").Handler(f(schemas, CreateTokenRequest))
	router.Methods("DELETE").Path("/v1-vault-driver/tokens").Handler(f(schemas, RevokeTokenRequest))

	return router
}
