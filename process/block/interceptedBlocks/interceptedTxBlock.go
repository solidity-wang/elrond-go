package interceptedBlocks

import (
	"github.com/ElrondNetwork/elrond-go/core"
	"github.com/ElrondNetwork/elrond-go/data/block"
	"github.com/ElrondNetwork/elrond-go/hashing"
	"github.com/ElrondNetwork/elrond-go/marshal"
	"github.com/ElrondNetwork/elrond-go/process"
	"github.com/ElrondNetwork/elrond-go/sharding"
)

// InterceptedTxBlockBody is a wrapper over a slice of miniblocks which contains transactions.
type InterceptedTxBlockBody struct {
	txBlockBody       *block.Body
	marshalizer       marshal.Marshalizer
	hasher            hashing.Hasher
	shardCoordinator  sharding.Coordinator
	hash              []byte
	isForCurrentShard bool
}

// NewInterceptedTxBlockBody creates a new instance of InterceptedTxBlockBody struct
func NewInterceptedTxBlockBody(arg *ArgInterceptedTxBlockBody) (*InterceptedTxBlockBody, error) {
	err := checkTxBlockBodyArgument(arg)
	if err != nil {
		return nil, err
	}

	txBlockBody, err := createTxBlockBody(arg.Marshalizer, arg.TxBlockBodyBuff)
	if err != nil {
		return nil, err
	}

	inTxBody := &InterceptedTxBlockBody{
		txBlockBody:      txBlockBody,
		marshalizer:      arg.Marshalizer,
		hasher:           arg.Hasher,
		shardCoordinator: arg.ShardCoordinator,
	}
	inTxBody.processFields(arg.TxBlockBodyBuff)

	return inTxBody, nil
}

func createTxBlockBody(marshalizer marshal.Marshalizer, txBlockBodyBuff []byte) (*block.Body, error) {
	txBlockBody := block.Body{}
	err := marshalizer.Unmarshal(&txBlockBody, txBlockBodyBuff)
	if err != nil {
		return nil, err
	}

	return &txBlockBody, nil
}

func (inTxBody *InterceptedTxBlockBody) processFields(txBuff []byte) {
	inTxBody.hash = inTxBody.hasher.Compute(string(txBuff))

	inTxBody.processIsForCurrentShard()
}

func (inTxBody *InterceptedTxBlockBody) processIsForCurrentShard() {
	inTxBody.isForCurrentShard = false
	for _, miniblock := range inTxBody.txBlockBody.MiniBlocks {
		inTxBody.isForCurrentShard = inTxBody.isMiniblockForCurrentShard(miniblock)
		if inTxBody.isForCurrentShard {
			return
		}
	}
}

func (inTxBody *InterceptedTxBlockBody) isMiniblockForCurrentShard(miniblock *block.MiniBlock) bool {
	isForCurrentShardRecv := miniblock.ReceiverShardID == inTxBody.shardCoordinator.SelfId()
	isForCurrentShardSender := miniblock.SenderShardID == inTxBody.shardCoordinator.SelfId()
	isForAllShards := miniblock.ReceiverShardID == core.AllShardId || miniblock.SenderShardID == core.AllShardId

	return isForCurrentShardRecv || isForCurrentShardSender || isForAllShards
}

// Hash gets the hash of this transaction block body
func (inTxBody *InterceptedTxBlockBody) Hash() []byte {
	return inTxBody.hash
}

// TxBlockBody returns the block body held by this wrapper
func (inTxBody *InterceptedTxBlockBody) TxBlockBody() *block.Body {
	return inTxBody.txBlockBody
}

// CheckValidity checks if the received tx block body is valid (not nil fields)
func (inTxBody *InterceptedTxBlockBody) CheckValidity() error {
	return inTxBody.integrity()
}

// IsForCurrentShard returns true if at least one contained miniblock is for current shard
func (inTxBody *InterceptedTxBlockBody) IsForCurrentShard() bool {
	return inTxBody.isForCurrentShard
}

// integrity checks the integrity of the tx block body
func (inTxBody *InterceptedTxBlockBody) integrity() error {
	for _, miniBlock := range inTxBody.txBlockBody.MiniBlocks {
		receiverNotCurrentShard := miniBlock.ReceiverShardID >= inTxBody.shardCoordinator.NumberOfShards() &&
			(miniBlock.ReceiverShardID != core.MetachainShardId &&
				miniBlock.ReceiverShardID != core.AllShardId)

		if receiverNotCurrentShard {
			return process.ErrInvalidShardId
		}

		senderNotCurrentShard := miniBlock.SenderShardID >= inTxBody.shardCoordinator.NumberOfShards() &&
			(miniBlock.SenderShardID != core.MetachainShardId &&
				miniBlock.SenderShardID != core.AllShardId)
		if senderNotCurrentShard {
			return process.ErrInvalidShardId
		}

		for _, txHash := range miniBlock.TxHashes {
			if txHash == nil {
				return process.ErrNilTxHash
			}
		}
	}

	return nil
}

// Type returns the type of this intercepted data
func (inTxBody *InterceptedTxBlockBody) Type() string {
	return "intercepted block body"
}

// IsInterfaceNil returns true if there is no value under the interface
func (inTxBody *InterceptedTxBlockBody) IsInterfaceNil() bool {
	return inTxBody == nil
}
