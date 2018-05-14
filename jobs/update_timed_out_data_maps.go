package jobs

import (
	"time"

	"github.com/getsentry/raven-go"
	"github.com/oysterprotocol/brokernode/models"
	"github.com/oysterprotocol/brokernode/utils"
	"gopkg.in/segmentio/analytics-go.v3"
)

func init() {
}

func UpdateTimeOutDataMaps(thresholdTime time.Time) {

	timedOutDataMaps := []models.DataMap{}

	err := models.DB.Where("status = ? AND updated_at <= ?", models.Unverified, thresholdTime).All(&timedOutDataMaps)
	if err != nil {
		raven.CaptureError(err, nil)
	}

	if len(timedOutDataMaps) > 0 {

		//when we bring back hooknodes, do decrement score somewhere in here

		for _, timedOutDataMap := range timedOutDataMaps {

			oyster_utils.LogToSegment("update_timed_out_data_maps: chunk_timed_out", analytics.NewProperties().
				Set("address", timedOutDataMap.Address).
				Set("genesis_hash", timedOutDataMap.GenesisHash).
				Set("chunk_idx", timedOutDataMap.ChunkIdx))

			timedOutDataMap.Status = models.Unassigned
			models.DB.ValidateAndSave(&timedOutDataMap)
		}
	}
}
