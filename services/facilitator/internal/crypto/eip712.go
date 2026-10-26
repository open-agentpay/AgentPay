package crypto

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/open-agentpay/agentpay-facilitator-go/internal/models"
)

var caip2Regex = regexp.MustCompile(`^eip155:([0-9]+)$`)

// EIP-712 TypeHashes
var (
	EIP712DomainTypeHash = crypto.Keccak256([]byte(
		"EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)",
	))

	TransferWithAuthorizationTypeHash = crypto.Keccak256([]byte(
		"TransferWithAuthorization(address from,address to,uint256 value,uint256 validAfter,uint256 validBefore,bytes32 nonce)",
	))
)

// DefaultTokenNames provides standard fallback token names for stablecoins.
var DefaultTokenNames = map[string]string{
	"usdc": "USD Coin",
	"eurc": "Euro Coin",
}

// ParseCAIP2ChainID extracts the chain ID from CAIP-2 network string (e.g. eip155:84532 -> 84532).
func ParseCAIP2ChainID(network string) (*big.Int, error) {
	match := caip2Regex.FindStringSubmatch(strings.TrimSpace(network))
	if len(match) < 2 {
		// If raw number passed
		if val, ok := new(big.Int).SetString(network, 10); ok {
			return val, nil
		}
		return nil, fmt.Errorf("unsupported or malformed CAIP-2 network identifier: %s", network)
	}
	id, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid chain ID in network %s: %w", network, err)
	}
	return big.NewInt(id), nil
}

// NormalizeBytes32 parses a hex or string nonce into a 32-byte array.
func NormalizeBytes32(nonce string) ([32]byte, error) {
	var b32 [32]byte
	clean := strings.TrimPrefix(strings.TrimSpace(nonce), "0x")
	if len(clean) > 64 {
		return b32, fmt.Errorf("nonce exceeds 32 bytes (64 hex characters): %s", nonce)
	}
	// Left pad to 64 hex characters if needed
	if len(clean) < 64 {
		clean = strings.Repeat("0", 64-len(clean)) + clean
	}
	bytesVal, err := hex.DecodeString(clean)
	if err != nil {
		return b32, fmt.Errorf("failed to decode hex nonce: %w", err)
	}
	copy(b32[:], bytesVal)
	return b32, nil
}

// ComputeDomainSeparator builds the EIP-712 domain separator hash.
func ComputeDomainSeparator(tokenName, tokenVersion string, chainID *big.Int, contract common.Address) [32]byte {
	nameHash := crypto.Keccak256([]byte(tokenName))
	versionHash := crypto.Keccak256([]byte(tokenVersion))

	var chainIDBytes [32]byte
	chainID.FillBytes(chainIDBytes[:])

	var contractBytes [32]byte
	copy(contractBytes[12:], contract.Bytes()) // 20 bytes address right-aligned

	data := make([]byte, 0, 32*5)
	data = append(data, EIP712DomainTypeHash...)
	data = append(data, nameHash...)
	data = append(data, versionHash...)
	data = append(data, chainIDBytes[:]...)
	data = append(data, contractBytes[:]...)

	var res [32]byte
	copy(res[:], crypto.Keccak256(data))
	return res
}

// ComputeTransferWithAuthorizationStructHash hashes the EIP-3009 transfer message.
func ComputeTransferWithAuthorizationStructHash(
	from common.Address,
	to common.Address,
	value *big.Int,
	validAfter int64,
	validBefore int64,
	nonce [32]byte,
) [32]byte {
	var fromBytes [32]byte
	copy(fromBytes[12:], from.Bytes())

	var toBytes [32]byte
	copy(toBytes[12:], to.Bytes())

	var valueBytes [32]byte
	if value != nil {
		value.FillBytes(valueBytes[:])
	}

	var validAfterBytes [32]byte
	new(big.Int).SetInt64(validAfter).FillBytes(validAfterBytes[:])

	var validBeforeBytes [32]byte
	new(big.Int).SetInt64(validBefore).FillBytes(validBeforeBytes[:])

	data := make([]byte, 0, 32*7)
	data = append(data, TransferWithAuthorizationTypeHash...)
	data = append(data, fromBytes[:]...)
	data = append(data, toBytes[:]...)
	data = append(data, valueBytes[:]...)
	data = append(data, validAfterBytes[:]...)
	data = append(data, validBeforeBytes[:]...)
	data = append(data, nonce[:]...)

	var res [32]byte
	copy(res[:], crypto.Keccak256(data))
	return res
}

// ComputeEIP712Digest produces the standard keccak256("\x19\x01" + domainSeparator + structHash).
func ComputeEIP712Digest(domainSeparator, structHash [32]byte) [32]byte {
	raw := make([]byte, 0, 2+32+32)
	raw = append(raw, 0x19, 0x01)
	raw = append(raw, domainSeparator[:]...)
	raw = append(raw, structHash[:]...)

	var digest [32]byte
	copy(digest[:], crypto.Keccak256(raw))
	return digest
}

// RecoverPayer extracts the signer address from an EIP-712 digest and signature.
func RecoverPayer(digest [32]byte, sig models.SignatureData) (common.Address, error) {
	var sigBytes []byte

	if sig.Serialized != "" {
		clean := strings.TrimPrefix(sig.Serialized, "0x")
		b, err := hex.DecodeString(clean)
		if err == nil && len(b) == 65 {
			sigBytes = b
		}
	}

	if len(sigBytes) == 0 && sig.R != "" && sig.S != "" {
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
			sigBytes = make([]byte, 65)
			copy(sigBytes[0:32], rb)
			copy(sigBytes[32:64], sb)
			sigBytes[64] = sig.V
		}
	}

	if len(sigBytes) != 65 {
		return common.Address{}, fmt.Errorf("invalid signature length, expected 65 bytes, got %d", len(sigBytes))
	}

	// In go-ethereum/crypto, SigToPub expects V in [0, 1]
	v := sigBytes[64]
	if v >= 27 {
		v -= 27
	}
	if v > 1 {
		return common.Address{}, fmt.Errorf("invalid signature recovery parameter V: %d", sigBytes[64])
	}
	sigBytes[64] = v

	pubKey, err := crypto.SigToPub(digest[:], sigBytes)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to recover public key from signature: %w", err)
	}

	return crypto.PubkeyToAddress(*pubKey), nil
}

// VerifyAuthorization validates whether auth satisfies challenge cryptographically.
func VerifyAuthorization(
	auth *models.PaymentAuthorization,
	ch *models.PaymentChallenge,
	tokenName string,
	tokenVersion string,
) (bool, string, error) {
	if auth == nil || ch == nil {
		return false, "", fmt.Errorf("nil authorization or challenge")
	}

	chainID, err := ParseCAIP2ChainID(ch.Network)
	if err != nil {
		return false, "", err
	}

	if tokenName == "" {
		symbol := strings.ToLower(ch.AssetSymbol)
		if name, ok := DefaultTokenNames[symbol]; ok {
			tokenName = name
		} else if ch.AssetSymbol != "" {
			tokenName = ch.AssetSymbol
		} else {
			tokenName = "USD Coin"
		}
	}

	if tokenVersion == "" {
		tokenVersion = "2"
	}

	contract := common.HexToAddress(ch.Asset)
	from := common.HexToAddress(auth.Payer)
	to := common.HexToAddress(ch.Payee)

	amountVal, ok := new(big.Int).SetString(ch.Amount, 10)
	if !ok {
		return false, "", fmt.Errorf("invalid challenge amount base units: %s", ch.Amount)
	}

	nonceBytes, err := NormalizeBytes32(auth.Nonce)
	if err != nil {
		return false, "", err
	}

	domainSep := ComputeDomainSeparator(tokenName, tokenVersion, chainID, contract)
	structHash := ComputeTransferWithAuthorizationStructHash(
		from,
		to,
		amountVal,
		auth.ValidAfter,
		auth.ValidBefore,
		nonceBytes,
	)
	digest := ComputeEIP712Digest(domainSep, structHash)

	recovered, err := RecoverPayer(digest, auth.Signature)
	if err != nil {
		return false, "", err
	}

	isValid := strings.EqualFold(recovered.Hex(), auth.Payer)
	return isValid, recovered.Hex(), nil
}
