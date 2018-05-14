package actions

import (
	"bytes"
	"encoding/json"
	"fmt"
	raven "github.com/getsentry/raven-go"
	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/nulls"
	"github.com/oysterprotocol/brokernode/models"
	"github.com/oysterprotocol/brokernode/services"
	"github.com/oysterprotocol/brokernode/utils"
	"gopkg.in/segmentio/analytics-go.v3"

	"github.com/pkg/errors"
	"math"
	"net/http"
	"strings"
	"time"
)

type UploadSessionResource struct {
	buffalo.Resource
}

// Request Response structs

type uploadSessionCreateReq struct {
	GenesisHash          string         `json:"genesisHash"`
	NumChunks            int            `json:"numChunks"`
	FileSizeBytes        int            `json:"fileSizeBytes"` // This is Trytes instead of Byte
	BetaIP               string         `json:"betaIp"`
	StorageLengthInYears int            `json:"storageLengthInYears"`
	AlphaTreasureIndexes []int          `json:"alphaTreasureIndexes"`
	Invoice              models.Invoice `json:"invoice"`
}

type uploadSessionCreateRes struct {
	ID            string               `json:"id"`
	UploadSession models.UploadSession `json:"uploadSession"`
	BetaSessionID string               `json:"betaSessionId"`
	Invoice       models.Invoice       `json:"invoice"`
}

type uploadSessionCreateBetaRes struct {
	ID                  string               `json:"id"`
	UploadSession       models.UploadSession `json:"uploadSession"`
	BetaSessionID       string               `json:"betaSessionId"`
	Invoice             models.Invoice       `json:"invoice"`
	BetaTreasureIndexes []int                `json:"betaTreasureIndexes"`
}

type chunkReq struct {
	Idx  int    `json:"idx"`
	Data string `json:"data"`
	Hash string `json:"hash"`
}

type UploadSessionUpdateReq struct {
	Chunks []chunkReq `json:"chunks"`
}

type paymentStatusCreateRes struct {
	ID            string `json:"id"`
	PaymentStatus string `json:"paymentStatus"`
}

const (
	SQL_BATCH_SIZE = 10
)

// Create creates an upload session.
func (usr *UploadSessionResource) Create(c buffalo.Context) error {

	req := uploadSessionCreateReq{}
	oyster_utils.ParseReqBody(c.Request(), &req)

	alphaEthAddr, privKey, _ := services.EthWrapper.GenerateEthAddr()

	// Start Alpha Session.
	alphaSession := models.UploadSession{
		Type:                 models.SessionTypeAlpha,
		GenesisHash:          req.GenesisHash,
		FileSizeBytes:        req.FileSizeBytes,
		NumChunks:            req.NumChunks,
		StorageLengthInYears: req.StorageLengthInYears,
		ETHAddrAlpha:         nulls.NewString(alphaEthAddr),
		ETHPrivateKey:        privKey,
	}

	defer oyster_utils.TimeTrack(time.Now(), "actions/upload_sessions: create_alpha_session", analytics.NewProperties().
		Set("id", alphaSession.ID).
		Set("genesis_hash", alphaSession.GenesisHash).
		Set("file_size_byes", alphaSession.FileSizeBytes).
		Set("num_chunks", alphaSession.NumChunks).
		Set("storage_years", alphaSession.StorageLengthInYears))

	vErr, err := alphaSession.StartUploadSession()
	if err != nil {
		return err
	}

	invoice := alphaSession.GetInvoice()

	// Mutates this because copying in golang sucks...
	req.Invoice = invoice

	req.AlphaTreasureIndexes = oyster_utils.GenerateInsertedIndexesForPearl(oyster_utils.ConvertToByte(req.FileSizeBytes))

	// Start Beta Session.
	var betaSessionID = ""
	var betaTreasureIndexes []int
	if req.BetaIP != "" {
		betaReq, err := json.Marshal(req)
		if err != nil {
			c.Render(400, r.JSON(map[string]string{"Error starting Beta": err.Error()}))
			return err
		}

		reqBetaBody := bytes.NewBuffer(betaReq)

		// Should we be hardcoding the port?
		betaURL := req.BetaIP + ":3000/api/v2/upload-sessions/beta"
		betaRes, err := http.Post(betaURL, "application/json", reqBetaBody)
		defer betaRes.Body.Close() // we need to close the connection
		if err != nil {
			c.Render(400, r.JSON(map[string]string{"Error starting Beta": err.Error()}))
			return err
		}
		betaSessionRes := &uploadSessionCreateBetaRes{}
		oyster_utils.ParseResBody(betaRes, betaSessionRes)
		betaSessionID = betaSessionRes.ID

		betaTreasureIndexes = betaSessionRes.BetaTreasureIndexes
	}

	err = models.DB.Save(&alphaSession)
	// Update alpha treasure idx map.
	alphaSession.MakeTreasureIdxMap(req.AlphaTreasureIndexes, betaTreasureIndexes)
	if err != nil {
		return err
	}

	if len(vErr.Errors) > 0 {
		c.Render(422, r.JSON(vErr.Errors))
		return err
	}

	res := uploadSessionCreateRes{
		UploadSession: alphaSession,
		ID:            alphaSession.ID.String(),
		BetaSessionID: betaSessionID,
		Invoice:       invoice,
	}

	return c.Render(200, r.JSON(res))
}

// Update uploads a chunk associated with an upload session.
func (usr *UploadSessionResource) Update(c buffalo.Context) error {

	req := UploadSessionUpdateReq{}
	oyster_utils.ParseReqBody(c.Request(), &req)

	// Get session
	uploadSession := &models.UploadSession{}
	err := models.DB.Find(uploadSession, c.Param("id"))

	defer oyster_utils.TimeTrack(time.Now(), "actions/upload_sessions: updating_session", analytics.NewProperties().
		Set("id", uploadSession.ID).
		Set("genesis_hash", uploadSession.GenesisHash).
		Set("file_size_byes", uploadSession.FileSizeBytes).
		Set("num_chunks", uploadSession.NumChunks).
		Set("storage_years", uploadSession.StorageLengthInYears))

	if err != nil || uploadSession == nil {
		c.Render(400, r.JSON(map[string]string{"Error finding session": errors.WithStack(err).Error()}))
		return err
	}
	treasureIdxMap, err := uploadSession.GetTreasureIndexes()

	// Update dMaps to have chunks async
	go func() {

		defer oyster_utils.TimeTrack(time.Now(), "actions/upload_sessions: async_datamap_updates", analytics.NewProperties().
			Set("id", uploadSession.ID).
			Set("genesis_hash", uploadSession.GenesisHash).
			Set("file_size_byes", uploadSession.FileSizeBytes).
			Set("num_chunks", uploadSession.NumChunks).
			Set("storage_years", uploadSession.StorageLengthInYears))

		var sqlWhereClosures []string
		chunksMap := make(map[string]chunkReq)
		minChunkIdx := float64(0)
		maxChunkIdx := float64(0)
		for _, chunk := range req.Chunks {
			var chunkIdx int
			if oyster_utils.BrokerMode == oyster_utils.TestModeNoTreasure {
				chunkIdx = chunk.Idx
			} else {
				chunkIdx = oyster_utils.TransformIndexWithBuriedIndexes(chunk.Idx, treasureIdxMap)
			}

			key := sqlWhereForGenesisHashAndChunkIdx(uploadSession.GenesisHash, chunkIdx)
			sqlWhereClosures = append(sqlWhereClosures, key)
			chunksMap[key] = chunk
			minChunkIdx = math.Min(minChunkIdx, float64(chunkIdx))
			maxChunkIdx = math.Max(maxChunkIdx, float64(chunkIdx))
		}

		var dms []models.DataMap
		//rawQuery := fmt.Sprintf("SELECT * from data_maps WHERE %s", strings.Join(sqlWhereClosures, " OR "))
		err := models.DB.RawQuery(
			"SELECT * from data_maps WHERE genesis_hash = ? AND chunk_idx >= ? AND chunk_idx <= ?",
			uploadSession.GenesisHash, minChunkIdx, maxChunkIdx).All(&dms)

		if err != nil {
			raven.CaptureError(err, nil)
		}

		dmsMap := make(map[string]models.DataMap)
		for _, dm := range dms {
			key := sqlWhereForGenesisHashAndChunkIdx(dm.GenesisHash, dm.ChunkIdx)
			// Only use the first one
			if _, hasKey := dmsMap[key]; !hasKey {
				dmsMap[key] = dm
			}
		}

		dbOperation, _ := oyster_utils.CreateDbUpdateOperation(&models.DataMap{})
		var updatedDms []string
		for key, chunk := range chunksMap {
			dm, hasKey := dmsMap[key]
			if !hasKey {
				continue
			}

			if chunk.Hash == dm.GenesisHash {
				dm.Message = chunk.Data
				if oyster_utils.BrokerMode == oyster_utils.TestModeNoTreasure {
					dm.Status = models.Unassigned
				}
				vErr, _ := dm.Validate(nil)
				if len(vErr.Errors) == 0 {
					updatedDms = append(updatedDms, fmt.Sprintf("(%s)", dbOperation.GetUpdatedValue(dm)))
				}
			}
		}

		numOfBatchRequest := int(math.Ceil(float64(len(updatedDms)) / float64(SQL_BATCH_SIZE)))

		remainder := len(updatedDms)
		for i := 0; i < numOfBatchRequest; i++ {
			lower := i * SQL_BATCH_SIZE
			upper := i*SQL_BATCH_SIZE + int(math.Min(float64(remainder), SQL_BATCH_SIZE))

			sectionUpdatedDms := updatedDms[lower:upper]

			// Do an insert operation and dup by primary key.

			rawQuery := fmt.Sprintf("INSERT INTO data_maps (%s) VALUES %s ON DUPLICATE KEY UPDATE message = VALUES(message), status = VALUES(status), updated_at = VALUES(updated_at)",
				dbOperation.GetColumns(), strings.Join(sectionUpdatedDms, ","))

			err = models.DB.RawQuery(rawQuery).All(&[]models.DataMap{})
			for err != nil {
				time.Sleep(300 * time.Millisecond)
				err = models.DB.RawQuery(rawQuery).All(&[]models.DataMap{})
			}

			remainder = remainder - SQL_BATCH_SIZE

			if err != nil {
				raven.CaptureError(err, nil)
				break
			}
		}
	}()

	return c.Render(202, r.JSON(map[string]bool{"success": true}))
}

// CreateBeta creates an upload session on the beta broker.
func (usr *UploadSessionResource) CreateBeta(c buffalo.Context) error {
	req := uploadSessionCreateReq{}
	oyster_utils.ParseReqBody(c.Request(), &req)

	betaTreasureIndexes := oyster_utils.GenerateInsertedIndexesForPearl(oyster_utils.ConvertToByte(req.FileSizeBytes))

	// Generates ETH address.
	betaEthAddr, privKey, _ := services.EthWrapper.GenerateEthAddr()

	u := models.UploadSession{
		Type:                 models.SessionTypeBeta,
		GenesisHash:          req.GenesisHash,
		NumChunks:            req.NumChunks,
		FileSizeBytes:        req.FileSizeBytes,
		StorageLengthInYears: req.StorageLengthInYears,
		TotalCost:            req.Invoice.Cost,
		ETHAddrAlpha:         req.Invoice.EthAddress,
		ETHAddrBeta:          nulls.NewString(betaEthAddr),
		ETHPrivateKey:        privKey,
	}

	defer oyster_utils.TimeTrack(time.Now(), "actions/upload_sessions: create_beta_session", analytics.NewProperties().
		Set("id", u.ID).
		Set("genesis_hash", u.GenesisHash).
		Set("file_size_byes", u.FileSizeBytes).
		Set("num_chunks", u.NumChunks).
		Set("storage_years", u.StorageLengthInYears))

	vErr, err := u.StartUploadSession()

	u.MakeTreasureIdxMap(req.AlphaTreasureIndexes, betaTreasureIndexes)
	if err != nil {
		return err
	}

	if len(vErr.Errors) > 0 {
		c.Render(422, r.JSON(vErr.Errors))
		return err
	}

	res := uploadSessionCreateBetaRes{
		UploadSession:       u,
		ID:                  u.ID.String(),
		Invoice:             u.GetInvoice(),
		BetaTreasureIndexes: betaTreasureIndexes,
	}
	return c.Render(200, r.JSON(res))
}

func (usr *UploadSessionResource) GetPaymentStatus(c buffalo.Context) error {
	session := models.UploadSession{}
	err := models.DB.Find(&session, c.Param("id"))

	if (err != nil || session == models.UploadSession{}) {
		//TODO: Return better error response when ID does not exist
		return err
	}

	res := paymentStatusCreateRes{
		ID:            session.ID.String(),
		PaymentStatus: session.GetPaymentStatus(),
	}

	return c.Render(200, r.JSON(res))
}

func sqlWhereForGenesisHashAndChunkIdx(genesisHash string, chunkIdx int) string {
	return fmt.Sprintf("(genesis_hash = '%s' AND chunk_idx = %d)", genesisHash, chunkIdx)
}
