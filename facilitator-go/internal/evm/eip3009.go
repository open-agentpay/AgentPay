package evm

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	agentcrypto "github.com/open-agentpay/agentpay-facilitator-go/internal/crypto"
	"github.com/open-agentpay/agentpay-facilitator-go/internal/models"
)

var (
	// TransferWithAuthorizationSelector = 0xe3ee160e
	TransferWithAuthorizationSelector = crypto.Keccak256([]byte("transferWithAuthorization(address,address,uint256,uint256,uint256,bytes32,uint8,bytes32,bytes32)"))[:4]

	// AuthorizationStateSelector = 0xe2973163
	AuthorizationStateSelector = crypto.Keccak256([]byte("authorizationState(address,bytes32)"))[:4]
)

// EncodeTransferWithAuthorization builds the 292-byte calldata for EIP-3009 transfer.
func EncodeTransferWithAuthorization(
	from common.Address,
	to common.Address,
	value *big.Int,
	validAfter int64,
	validBefore int64,
	nonce [32]byte,
	v uint8,
	r [32]byte,
	s [32]byte,
) []byte {
	calldata := make([]byte, 4+32*9)
	copy(calldata[0:4], TransferWithAuthorizationSelector)

	offset := 4

	// 1. from
	copy(calldata[offset+12:offset+32], from.Bytes())
	offset += 32

	// 2. to
	copy(calldata[offset+12:offset+32], to.Bytes())
	offset += 32

	// 3. value
	if value != nil {
		value.FillBytes(calldata[offset : offset+32])
	}
	offset += 32

	// 4. validAfter
	new(big.Int).SetInt64(validAfter).FillBytes(calldata[offset : offset+32])
	offset += 32

	// 5. validBefore
	new(big.Int).SetInt64(validBefore).FillBytes(calldata[offset : offset+32])
	offset += 32

	// 6. nonce
	copy(calldata[offset:offset+32], nonce[:])
	offset += 32

	// 7. v (uint8 right-aligned in 32-byte slot)
	calldata[offset+31] = v
	offset += 32

	// 8. r
	copy(calldata[offset:offset+32], r[:])
	offset += 32

	// 9. s
	copy(calldata[offset:offset+32], s[:])

	return calldata
}

// EncodeTransferFromAuthorization converts models.PaymentAuthorization into calldata.
func EncodeTransferFromAuthorization(
	auth *models.PaymentAuthorization,
	payee common.Address,
	amount *big.Int,
) ([]byte, error) {
	if auth == nil {
		return nil, fmt.Errorf("nil payment authorization")
	}

	from := common.HexToAddress(auth.Payer)
	nonceBytes, err := agentcrypto.NormalizeBytes32(auth.Nonce)
	if err != nil {
		return nil, err
	}

	var rBytes, sBytes [32]byte
	var v uint8

	sig := auth.Signature
	if sig.Serialized != "" {
		clean := strings.TrimPrefix(sig.Serialized, "0x")
		b, err := hex.DecodeString(clean)
		if err == nil && len(b) == 65 {
			copy(rBytes[:], b[0:32])
			copy(sBytes[:], b[32:64])
			v = b[64]
		}
	}

	if v == 0 && sig.R != "" && sig.S != "" {
		cleanR := strings.TrimPrefix(sig.R, "0x")
		cleanS := strings.TrimPrefix(sig.S, "0x")
		if len(cleanR) < 64 {
			cleanR = strings.Repeat("0", 64-len(cleanR)) + cleanR
		}
		if len(cleanS) < 64 {
			cleanS = strings.Repeat("0", 64-len(cleanS)) + cleanS
		}
		rb, errR := hex.DecodeString(cleanR)
		sb, errS := hex.DecodeString(cleanS)
		if errR == nil && errS == nil && len(rb) == 32 && len(sb) == 32 {
			copy(rBytes[:], rb)
			copy(sBytes[:], sb)
			v = sig.V
		}
	}

	if v == 0 {
		return nil, fmt.Errorf("invalid signature parameters")
	}

	return EncodeTransferWithAuthorization(
		from,
		payee,
		amount,
		auth.ValidAfter,
		auth.ValidBefore,
		nonceBytes,
		v,
		rBytes,
		sBytes,
	), nil
}

// EncodeAuthorizationState builds the 68-byte calldata for authorizationState(address,bytes32).
func EncodeAuthorizationState(authorizer common.Address, nonce [32]byte) []byte {
	calldata := make([]byte, 4+32*2)
	copy(calldata[0:4], AuthorizationStateSelector)

	// 1. authorizer
	copy(calldata[4+12:36], authorizer.Bytes())

	// 2. nonce
	copy(calldata[36:68], nonce[:])

	return calldata
}

// DecodeAuthorizationStateResult parses the boolean returned by authorizationState.
func DecodeAuthorizationStateResult(data []byte) bool {
	if len(data) < 32 {
		return false
	}
	// If any byte in the 32-byte word is non-zero, it means authorization is already used/cancelled
	for _, b := range data[:32] {
		if b != 0 {
			return true
		}
	}
	return false
}
