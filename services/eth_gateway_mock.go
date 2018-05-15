package services

import (
	"github.com/oysterprotocol/brokernode/models"
)

var EthMock Eth

func init() {
	SetUpMock()
}

func SetUpMock() {

	EthMock = Eth{
		ClaimUnusedPRLs: func(completedUploads []models.CompletedUpload) error {
			return nil
		},
		SendGas: func([]models.CompletedUpload) error {
			return nil
		},
	}
}
