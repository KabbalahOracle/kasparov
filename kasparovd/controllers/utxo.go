package controllers

import (
	"encoding/hex"
	"fmt"

	"github.com/kaspanet/kaspad/util/subnetworkid"
	"github.com/kaspanet/kasparov/apimodels"
	"github.com/kaspanet/kasparov/dbaccess"
	"github.com/kaspanet/kasparov/dbmodels"
	"github.com/kaspanet/kasparov/kasparovd/config"
	"github.com/pkg/errors"
)

// GetUTXOsByAddressHandler searches for all UTXOs that belong to a certain address.
func GetUTXOsByAddressHandler(address string) (interface{}, error) {
	if err := validateAddress(address); err != nil {
		return nil, err
	}

	transactionOutputs, err := dbaccess.UTXOsByAddress(dbaccess.NoTx(), address,
		dbmodels.TransactionOutputFieldNames.TransactionAcceptingBlock,
		dbmodels.TransactionOutputFieldNames.TransactionSubnetwork)
	if err != nil {
		return nil, err
	}

	nonAcceptedTxIds := make([]uint64, len(transactionOutputs))
	for i, txOut := range transactionOutputs {
		if txOut.Transaction.AcceptingBlock == nil {
			nonAcceptedTxIds[i] = txOut.TransactionID
		}
	}

	selectedTipBlueScore, err := dbaccess.SelectedTipBlueScore(dbaccess.NoTx())
	if err != nil {
		return nil, err
	}
	activeNetParams := config.ActiveConfig().NetParams()

	UTXOsResponses := make([]*apimodels.TransactionOutputResponse, len(transactionOutputs))
	for i, transactionOutput := range transactionOutputs {
		subnetworkID := &subnetworkid.SubnetworkID{}
		err := subnetworkid.Decode(subnetworkID, transactionOutput.Transaction.Subnetwork.SubnetworkID)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("Couldn't decode subnetwork id %s", transactionOutput.Transaction.Subnetwork.SubnetworkID))
		}
		var acceptingBlockHash *string
		var acceptingBlockBlueScore *uint64
		if transactionOutput.Transaction.AcceptingBlock != nil {
			acceptingBlockHash = &transactionOutput.Transaction.AcceptingBlock.BlockHash
			acceptingBlockBlueScore = &transactionOutput.Transaction.AcceptingBlock.BlueScore
		}
		isCoinbase := subnetworkID.IsEqual(subnetworkid.SubnetworkIDCoinbase)
		utxoConfirmations := confirmations(acceptingBlockBlueScore, selectedTipBlueScore)
		isSpendable := (!isCoinbase && utxoConfirmations > 0) ||
			(isCoinbase && utxoConfirmations >= activeNetParams.BlockCoinbaseMaturity)

		UTXOsResponses[i] = &apimodels.TransactionOutputResponse{
			TransactionID:           transactionOutput.Transaction.TransactionID,
			Value:                   transactionOutput.Value,
			ScriptPubKey:            hex.EncodeToString(transactionOutput.ScriptPubKey),
			AcceptingBlockHash:      acceptingBlockHash,
			AcceptingBlockBlueScore: acceptingBlockBlueScore,
			Index:                   transactionOutput.Index,
			IsCoinbase:              &isCoinbase,
			Confirmations:           &utxoConfirmations,
			IsSpendable:             &isSpendable,
		}
	}
	return UTXOsResponses, nil
}
