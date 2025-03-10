package push

import (
	"encoding/json"
	"net/http"

	"github.com/citizenwallet/engine/internal/db"
	com "github.com/citizenwallet/engine/pkg/common"
	"github.com/citizenwallet/engine/pkg/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-chi/chi/v5"
)

type Service struct {
	db *db.DB
}

func NewService(db *db.DB) *Service {
	return &Service{
		db: db,
	}
}

func (s *Service) AddToken(w http.ResponseWriter, r *http.Request) {
	// ensure that the address in the url matches the one in the headers
	addr, ok := com.GetContextAddress(r.Context())
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	haccaddr := common.HexToAddress(addr)

	// parse address from url params
	accaddr := chi.URLParam(r, "acc_addr")

	acc := common.HexToAddress(accaddr)

	if haccaddr != acc {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// parse contract address from url params
	contractAddr := chi.URLParam(r, "contract_address")

	var pt engine.PushToken
	err := json.NewDecoder(r.Body).Decode(&pt)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// make sure the addresses are EIP55 checksummed
	pt.Account = com.ChecksumAddress(pt.Account)

	// check that the push token is from the sender of the transaction
	if !com.IsSameHexAddress(pt.Account, acc.Hex()) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	tname, err := s.db.TableNameSuffix(contractAddr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	pdb, ok := s.db.PushTokenDB[tname]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	err = pdb.AddToken(&pt)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = com.Body(w, pt, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *Service) RemoveAccountToken(w http.ResponseWriter, r *http.Request) {
	// ensure that the address in the url matches the one in the headers
	addr, ok := com.GetContextAddress(r.Context())
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	haccaddr := common.HexToAddress(addr)

	// parse address from url params
	accaddr := chi.URLParam(r, "acc_addr")

	acc := common.HexToAddress(accaddr)

	if haccaddr != acc {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// parse contract address from url params
	contractAddr := chi.URLParam(r, "contract_address")

	// parse token from url params
	token := chi.URLParam(r, "token")

	if token == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tname, err := s.db.TableNameSuffix(contractAddr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	pdb, ok := s.db.PushTokenDB[tname]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	err = pdb.RemoveAccountPushToken(token, accaddr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = com.Body(w, []byte("{}"), nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
