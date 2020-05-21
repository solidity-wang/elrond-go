package mock

import (
	"sync"

	"github.com/ElrondNetwork/elrond-go/crypto"
)

// CryptoComponentsMock -
type CryptoComponentsMock struct {
	PubKey      crypto.PublicKey
	BlockSig    crypto.SingleSigner
	TxSig       crypto.SingleSigner
	MultiSig    crypto.MultiSigner
	BlKeyGen    crypto.KeyGenerator
	TxKeyGen    crypto.KeyGenerator
	mutMultiSig sync.RWMutex
}

// PublicKey -
func (ccm *CryptoComponentsMock) PublicKey() crypto.PublicKey {
	return ccm.PubKey
}

// BlockSigner -
func (ccm *CryptoComponentsMock) BlockSigner() crypto.SingleSigner {
	return ccm.BlockSig
}

// TxSingleSigner -
func (ccm *CryptoComponentsMock) TxSingleSigner() crypto.SingleSigner {
	return ccm.TxSig
}

// MultiSigner -
func (ccm *CryptoComponentsMock) MultiSigner() crypto.MultiSigner {
	ccm.mutMultiSig.RLock()
	defer ccm.mutMultiSig.RUnlock()

	return ccm.MultiSig
}

// SetMultiSigner -
func (ccm *CryptoComponentsMock) SetMultiSigner(ms crypto.MultiSigner) error {
	ccm.mutMultiSig.Lock()
	ccm.MultiSig = ms
	ccm.mutMultiSig.Unlock()

	return nil
}

// BlockSignKeyGen -
func (ccm *CryptoComponentsMock) BlockSignKeyGen() crypto.KeyGenerator {
	return ccm.BlKeyGen
}

// TxSignKeyGen -
func (ccm *CryptoComponentsMock) TxSignKeyGen() crypto.KeyGenerator {
	return ccm.TxKeyGen
}

// IsInterfaceNil -
func (ccm *CryptoComponentsMock) IsInterfaceNil() bool {
	return ccm == nil
}
