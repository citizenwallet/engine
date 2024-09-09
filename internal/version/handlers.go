package version

import (
	"net/http"

	"github.com/citizenwallet/engine/pkg/common"
	"github.com/citizenwallet/engine/pkg/engine"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

type response struct {
	Version string `json:"version"`
}

// Current returns the current version of the API
func (s *Service) Current(w http.ResponseWriter, r *http.Request) {
	err := common.Body(w, &response{Version: engine.Version}, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
