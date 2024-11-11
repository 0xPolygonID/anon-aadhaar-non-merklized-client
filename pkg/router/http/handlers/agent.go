package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/iden3/driver-did-iden3/pkg/services/blockchain/eth"
	"github.com/iden3/iden3comm/v2"
	"github.com/iden3/iden3comm/v2/packers"
	"github.com/iden3/iden3comm/v2/protocol"
	"github.com/pkg/errors"
)

type AgentHandlers struct {
	packer          *iden3comm.PackageManager
	verificaitonURL string
}

func NewAgentHandlers(authV2VerificationKeyPath string, verificationURL string) (AgentHandlers, error) {
	verificationKey, err := os.ReadFile(authV2VerificationKeyPath)
	if err != nil {
		return AgentHandlers{}, fmt.Errorf("failed to read verification key: %v", err)
	}

	r, err := eth.NewResolver("https://rpc-mainnet.privado.id", "0x3C9acB2205Aa72A05F6D77d708b5Cf85FCa3a896")
	if err != nil {
		return AgentHandlers{}, errors.Errorf("failed to create resolver: %v", err)
	}
	resolvers := map[int]eth.Resolver{
		21000: *r,
	}
	zkpUnpacker := packers.DefaultZKPUnpacker(
		verificationKey, resolvers, packers.WithAuthVerifyDelay(time.Minute*15))

	packageManager := iden3comm.NewPackageManager()
	if err := packageManager.RegisterPackers(zkpUnpacker); err != nil {
		return AgentHandlers{}, errors.Errorf("failed to register zkp unpacker: %v", err)
	}

	return AgentHandlers{
		packer:          packageManager,
		verificaitonURL: verificationURL,
	}, nil
}

func (h *AgentHandlers) Agent(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read payload", http.StatusBadRequest)
		return
	}
	basicMessage, _, err := h.packer.Unpack(payload)
	if err != nil {
		http.Error(w, "failed to unpack payload", http.StatusBadRequest)
		return
	}
	u := fmt.Sprintf("%s/claim?userID=%s&issuerID=%s", h.verificaitonURL, basicMessage.From, basicMessage.To)
	credentialProposal := buildCredentialProposalResponse(u)
	credentialProposalBytes, err := json.Marshal(credentialProposal)
	if err != nil {
		http.Error(w, "failed to marshal credential proposal", http.StatusInternalServerError)
		return
	}

	response, err := json.Marshal(&iden3comm.BasicMessage{
		ID:       basicMessage.ID,
		Typ:      packers.MediaTypePlainMessage,
		Type:     protocol.CredentialProposalMessageType,
		ThreadID: basicMessage.ThreadID,
		From:     basicMessage.To,
		To:       basicMessage.From,
		Body:     credentialProposalBytes,
	})
	if err != nil {
		http.Error(w, "failed to marshal response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

func buildCredentialProposalResponse(url string) *protocol.CredentialsProposalBody {
	return &protocol.CredentialsProposalBody{
		Proposals: []protocol.CredentialProposalInfo{
			{
				Credentials: []protocol.CredentialInfo{{
					Context: "https://raw.githubusercontent.com/anon-aadhaar/privado-contracts/main/assets/anon-aadhaar.jsonld",
					Type:    "AnonAadhaarCredential",
				}},
				Type:        "anon-aadhaar",
				URL:         url,
				Description: "anon aadhaar credential",
			},
		},
	}
}
