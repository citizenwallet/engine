package engine

import "net/http"

type RPCHandlerFunc func(r *http.Request) (any, error)
