package chain

import (
	"math/big"
	"net/http"

	"github.com/citizenwallet/engine/pkg/engine"
)

type Service struct {
	evm     engine.EVMRequester
	chainId *big.Int
}

// NewService
func NewService(evm engine.EVMRequester, chid *big.Int) *Service {
	return &Service{
		evm,
		chid,
	}
}

func (s *Service) ChainId(r *http.Request) (any, int) {
	// Return the message ID
	return s.chainId, http.StatusOK
}
