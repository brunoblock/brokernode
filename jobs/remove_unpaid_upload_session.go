package jobs

import (
	"github.com/gobuffalo/pop"
	"github.com/oysterprotocol/brokernode/models"
	"github.com/oysterprotocol/brokernode/services"
	"github.com/oysterprotocol/brokernode/utils"
)

/*UnpaidExpirationInHour means number of hours before it should remove unpaid upload session. */
const UnpaidExpirationInHour = 24

/*RemoveUnpaidUploadSession cleans up unpload_sessions and data_maps talbe for expired/unpaid session. */
func RemoveUnpaidUploadSession() {
	sessions := []models.UploadSession{}
	err := models.DB.RawQuery(
		"SELECT * from upload_sessions WHERE payment_status != ? AND TIMESTAMPDIFF(hour, updated_at, NOW()) >= ?",
		models.PaymentStatusConfirmed, UnpaidExpirationInHour).All(&sessions)
	if err != nil {
		oyster_utils.LogIfError(err)
		return
	}

	for _, session := range sessions {
		balance := EthWrapper.CheckBalance(services.StringToAddress(session.ETHAddrAlpha.String))
		if balance.Int64() > 0 {
			continue
		}

		err := models.DB.Transaction(func(tx *pop.Connection) error {
			if err := tx.RawQuery("DELETE from data_maps WHERE genesis_hash = ?", session.GenesisHash).All(&[]models.DataMap{}); err != nil {
				return nil
			}
			if err := tx.RawQuery("DELETE from upload_sessions WHERE id = ?", session.ID).All(&[]models.UploadSession{}); err != nil {
				return err
			}
			return nil
		})
		oyster_utils.LogIfError(err)
	}
}
