package actions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/nulls"
	"github.com/oysterprotocol/brokernode/models"
	"github.com/oysterprotocol/brokernode/services"
	"github.com/oysterprotocol/brokernode/utils"
	"github.com/pkg/errors"
	"gopkg.in/segmentio/analytics-go.v3"
)

type UploadSessionResource struct {
	buffalo.Resource
}

// Request Response structs

type uploadSessionCreateReq struct {
	GenesisHash          string         `json:"genesisHash"`
	NumChunks            int            `json:"numChunks"`
	FileSizeBytes        uint64         `json:"fileSizeBytes"` // This is Trytes instead of Byte
	BetaIP               string         `json:"betaIp"`
	StorageLengthInYears int            `json:"storageLengthInYears"`
	AlphaTreasureIndexes []int          `json:"alphaTreasureIndexes"`
	Invoice              models.Invoice `json:"invoice"`
	Version              uint32         `json:"version"`
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
	Hash string `json:"hash"` // This is GenesisHash.
}

type UploadSessionUpdateReq struct {
	Chunks []chunkReq `json:"chunks"`
}

type paymentStatusCreateRes struct {
	ID            string `json:"id"`
	PaymentStatus string `json:"paymentStatus"`
}

var NumChunksLimit = -1 //unlimited

func init() {

}

// Create creates an upload session.
func (usr *UploadSessionResource) Create(c buffalo.Context) error {

	if os.Getenv("DEPLOY_IN_PROGRESS") == "true" {
		err := errors.New("Deployment in progress.  Try again later")
		fmt.Println(err)
		c.Error(400, err)
		return err
	}

	if v, err := strconv.Atoi(os.Getenv("NUM_CHUNKS_LIMIT")); err == nil {
		NumChunksLimit = v
	}

	start := PrometheusWrapper.TimeNow()
	defer PrometheusWrapper.HistogramSeconds(PrometheusWrapper.HistogramUploadSessionResourceCreate, start)

	req := uploadSessionCreateReq{}
	if err := oyster_utils.ParseReqBody(c.Request(), &req); err != nil {
		err = fmt.Errorf("Invalid request, unable to parse request body  %v", err)
		c.Error(400, err)
		return err
	}

	if NumChunksLimit != -1 && req.NumChunks > NumChunksLimit {
		err := errors.New("This broker has a limit of " + fmt.Sprint(NumChunksLimit) + " file chunks.")
		fmt.Println(err)
		c.Error(400, err)
		return err
	}

	alphaEthAddr, privKey, _ := EthWrapper.GenerateEthAddr()

	// Start Alpha Session.
	alphaSession := models.UploadSession{
		Type:                 models.SessionTypeAlpha,
		GenesisHash:          req.GenesisHash,
		FileSizeBytes:        req.FileSizeBytes,
		NumChunks:            req.NumChunks,
		StorageLengthInYears: req.StorageLengthInYears,
		ETHAddrAlpha:         nulls.NewString(alphaEthAddr.Hex()),
		ETHPrivateKey:        privKey,
		Version:              req.Version,
	}

	defer oyster_utils.TimeTrack(time.Now(), "actions/upload_sessions: create_alpha_session", analytics.NewProperties().
		Set("id", alphaSession.ID).
		Set("genesis_hash", alphaSession.GenesisHash).
		Set("file_size_byes", alphaSession.FileSizeBytes).
		Set("num_chunks", alphaSession.NumChunks).
		Set("storage_years", alphaSession.StorageLengthInYears))

	vErr, err := alphaSession.StartUploadSession()
	if err != nil || vErr.HasAny() {
		err = fmt.Errorf("StartUploadSession error: %v and validation error: %v", err, vErr)
		c.Error(400, err)
		return err
	}

	invoice := alphaSession.GetInvoice()

	// Mutates this because copying in golang sucks...
	req.Invoice = invoice

	req.AlphaTreasureIndexes = oyster_utils.GenerateInsertedIndexesForPearl(oyster_utils.ConvertToByte(req.FileSizeBytes))

	// Start Beta Session.
	var betaSessionID = ""
	var betaTreasureIndexes []int
	hasBeta := req.BetaIP != ""
	if hasBeta {
		betaReq, err := json.Marshal(req)
		if err != nil {
			oyster_utils.LogIfError(err, nil)
			c.Error(400, err)
			return err
		}

		reqBetaBody := bytes.NewBuffer(betaReq)

		// Should we be hardcoding the port?
		betaURL := req.BetaIP + ":3000/api/v2/upload-sessions/beta"
		betaRes, err := http.Post(betaURL, "application/json", reqBetaBody)
		defer betaRes.Body.Close() // we need to close the connection
		if err != nil {
			oyster_utils.LogIfError(err, nil)
			c.Error(400, err)
			return err
		}
		betaSessionRes := &uploadSessionCreateBetaRes{}
		if err := oyster_utils.ParseResBody(betaRes, betaSessionRes); err != nil {
			err = fmt.Errorf("Unable to communicate with Beta node: %v", err)
			// This should consider as BadRequest since the client pick the beta node.
			c.Error(400, err)
			return err
		}

		betaSessionID = betaSessionRes.ID

		betaTreasureIndexes = betaSessionRes.BetaTreasureIndexes
		alphaSession.ETHAddrBeta = betaSessionRes.UploadSession.ETHAddrBeta
	}

	if err := models.DB.Save(&alphaSession); err != nil {
		oyster_utils.LogIfError(err, nil)
		c.Error(400, err)
		return err
	}

	models.NewBrokerBrokerTransaction(&alphaSession)

	if hasBeta {
		mergedIndexes, _ := oyster_utils.MergeIndexes(req.AlphaTreasureIndexes, betaTreasureIndexes, oyster_utils.FileSectorInChunkSize, req.NumChunks)

		privateKeys, err := EthWrapper.GenerateKeys(len(mergedIndexes))
		if err != nil {
			err := errors.New("Could not generate eth keys: " + err.Error())
			fmt.Println(err)
			c.Error(400, err)
			return err
		}
		if len(mergedIndexes) != len(privateKeys) {
			err := errors.New("privateKeys and mergedIndexes should have the same length")
			oyster_utils.LogIfError(err, nil)
			c.Error(400, err)
			return err
		}
		// Update alpha treasure idx map.
		alphaSession.MakeTreasureIdxMap(mergedIndexes, privateKeys)
	}

	res := uploadSessionCreateRes{
		UploadSession: alphaSession,
		ID:            alphaSession.ID.String(),
		BetaSessionID: betaSessionID,
		Invoice:       invoice,
	}
	//go waitForTransferAndNotifyBeta(
	//	res.UploadSession.ETHAddrAlpha.String, res.UploadSession.ETHAddrBeta.String, res.ID)

	return c.Render(200, r.JSON(res))
}

// Update uploads a chunk associated with an upload session.
func (usr *UploadSessionResource) Update(c buffalo.Context) error {
	start := PrometheusWrapper.TimeNow()
	defer PrometheusWrapper.HistogramSeconds(PrometheusWrapper.HistogramUploadSessionResourceUpdate, start)

	req := UploadSessionUpdateReq{}
	if err := oyster_utils.ParseReqBody(c.Request(), &req); err != nil {
		err = fmt.Errorf("Invalid request, unable to parse request body  %v", err)
		c.Error(400, err)
		return err
	}

	// Get session
	uploadSession := &models.UploadSession{}
	err := models.DB.Find(uploadSession, c.Param("id"))

	defer oyster_utils.TimeTrack(time.Now(), "actions/upload_sessions: updating_session", analytics.NewProperties().
		Set("id", uploadSession.ID).
		Set("genesis_hash", uploadSession.GenesisHash).
		Set("file_size_byes", uploadSession.FileSizeBytes).
		Set("num_chunks", uploadSession.NumChunks).
		Set("storage_years", uploadSession.StorageLengthInYears))

	if err != nil {
		oyster_utils.LogIfError(err, nil)
		c.Error(400, err)
		return err
	}
	if uploadSession == nil {
		err := errors.New("Error finding sessions")
		oyster_utils.LogIfError(err, nil)
		c.Error(400, err)
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

		ProcessAndStoreChunkData(req.Chunks, uploadSession.GenesisHash, treasureIdxMap)
	}()

	return c.Render(202, r.JSON(map[string]bool{"success": true}))
}

// CreateBeta creates an upload session on the beta broker.
func (usr *UploadSessionResource) CreateBeta(c buffalo.Context) error {
	start := PrometheusWrapper.TimeNow()
	defer PrometheusWrapper.HistogramSeconds(PrometheusWrapper.HistogramUploadSessionResourceCreateBeta, start)

	req := uploadSessionCreateReq{}
	if err := oyster_utils.ParseReqBody(c.Request(), &req); err != nil {
		err = fmt.Errorf("Invalid request, unable to parse request body  %v", err)
		c.Error(400, err)
		return err
	}

	betaTreasureIndexes := oyster_utils.GenerateInsertedIndexesForPearl(oyster_utils.ConvertToByte(req.FileSizeBytes))

	// Generates ETH address.
	betaEthAddr, privKey, _ := EthWrapper.GenerateEthAddr()

	u := models.UploadSession{
		Type:                 models.SessionTypeBeta,
		GenesisHash:          req.GenesisHash,
		NumChunks:            req.NumChunks,
		FileSizeBytes:        req.FileSizeBytes,
		StorageLengthInYears: req.StorageLengthInYears,
		TotalCost:            req.Invoice.Cost,
		ETHAddrAlpha:         req.Invoice.EthAddress,
		ETHAddrBeta:          nulls.NewString(betaEthAddr.Hex()),
		ETHPrivateKey:        privKey,
		Version:              req.Version,
	}

	defer oyster_utils.TimeTrack(time.Now(), "actions/upload_sessions: create_beta_session", analytics.NewProperties().
		Set("id", u.ID).
		Set("genesis_hash", u.GenesisHash).
		Set("file_size_byes", u.FileSizeBytes).
		Set("num_chunks", u.NumChunks).
		Set("storage_years", u.StorageLengthInYears))

	vErr, err := u.StartUploadSession()

	if err != nil || vErr.HasAny() {
		err = fmt.Errorf("Can't startUploadSession with validation error: %v and err: %v", vErr, err)
		c.Error(400, err)
		return err
	}

	models.NewBrokerBrokerTransaction(&u)

	mergedIndexes, err := oyster_utils.MergeIndexes(req.AlphaTreasureIndexes, betaTreasureIndexes, oyster_utils.FileSectorInChunkSize, req.NumChunks)
	if err != nil {
		fmt.Println(err)
		c.Error(400, err)
		return err
	}
	privateKeys, err := EthWrapper.GenerateKeys(len(mergedIndexes))
	if err != nil {
		err := errors.New("Could not generate eth keys: " + err.Error())
		fmt.Println(err)
		c.Error(400, err)
		return err
	}
	if len(mergedIndexes) != len(privateKeys) {
		err := errors.New("privateKeys and mergedIndexes should have the same length")
		fmt.Println(err)
		c.Error(400, err)
		return err
	}
	u.MakeTreasureIdxMap(mergedIndexes, privateKeys)

	res := uploadSessionCreateBetaRes{
		UploadSession:       u,
		ID:                  u.ID.String(),
		Invoice:             u.GetInvoice(),
		BetaTreasureIndexes: betaTreasureIndexes,
	}
	//go waitForTransferAndNotifyBeta(
	//	res.UploadSession.ETHAddrAlpha.String, res.UploadSession.ETHAddrBeta.String, res.ID)

	return c.Render(200, r.JSON(res))
}

func (usr *UploadSessionResource) GetPaymentStatus(c buffalo.Context) error {
	start := PrometheusWrapper.TimeNow()
	defer PrometheusWrapper.HistogramSeconds(PrometheusWrapper.HistogramUploadSessionResourceGetPaymentStatus, start)

	session := models.UploadSession{}
	err := models.DB.Find(&session, c.Param("id"))

	if err != nil {
		c.Error(400, err)
		oyster_utils.LogIfError(err, nil)
		return err
	}
	if (session == models.UploadSession{}) {
		err := errors.New("Did not find session that matched id" + c.Param("id"))
		oyster_utils.LogIfError(err, nil)
		c.Error(400, err)
		return err
	}

	// Force to check the status
	if session.PaymentStatus != models.PaymentStatusConfirmed {
		balance := EthWrapper.CheckPRLBalance(services.StringToAddress(session.ETHAddrAlpha.String))
		if balance.Int64() > 0 {
			previousPaymentStatus := session.PaymentStatus
			session.PaymentStatus = models.PaymentStatusConfirmed
			err = models.DB.Save(&session)
			if err != nil {
				session.PaymentStatus = previousPaymentStatus
			} else {
				models.SetBrokerTransactionToPaid(session)
			}
		}
	}

	res := paymentStatusCreateRes{
		ID:            session.ID.String(),
		PaymentStatus: session.GetPaymentStatus(),
	}

	return c.Render(200, r.JSON(res))
}

/*ProcessAndStoreChunkData receives the genesis hash, chunk idx, and message from the client
and adds it to the badger database*/
func ProcessAndStoreChunkData(chunks []chunkReq, genesisHash string, treasureIdxMap []int) {
	// the keys in this chunks map have already transformed indexes
	chunksMap := convertToBadgerKeyedMapForChunks(chunks, genesisHash, treasureIdxMap)

	batchSetKvMap := services.KVPairs{} // Store chunk.Data into KVStore
	for key, chunk := range chunksMap {

		batchSetKvMap[key] = chunk.Data
	}

	services.BatchSet(&batchSetKvMap, models.DataMapsTimeToLive)
}

// convertToBadgerKeyedMapForChunks converts chunkReq into maps where the key is the badger msg_id.
// Return minChunkId and maxChunkId.
func convertToBadgerKeyedMapForChunks(chunks []chunkReq, genesisHash string, treasureIdxMap []int) map[string]chunkReq {
	chunksMap := make(map[string]chunkReq)

	for _, chunk := range chunks {
		var chunkIdx int
		if oyster_utils.BrokerMode == oyster_utils.TestModeNoTreasure {
			chunkIdx = chunk.Idx
		} else {
			chunkIdx = oyster_utils.TransformIndexWithBuriedIndexes(chunk.Idx, treasureIdxMap)
		}

		key := oyster_utils.GenerateBadgerKey("", genesisHash, chunkIdx)
		chunksMap[key] = chunk
	}
	return chunksMap
}

// convertToSQLKeyedMapForChunks converts chunkReq into sql keyed maps. Return minChunkId and maxChunkId.
func convertToSQLKeyedMapForChunks(chunks []chunkReq, genesisHash string, treasureIdxMap []int) (map[string]chunkReq, int, int) {
	chunksMap := make(map[string]chunkReq)
	minChunkIdx := 0
	maxChunkIdx := 0

	for _, chunk := range chunks {
		var chunkIdx int
		if oyster_utils.BrokerMode == oyster_utils.TestModeNoTreasure {
			chunkIdx = chunk.Idx
		} else {
			chunkIdx = oyster_utils.TransformIndexWithBuriedIndexes(chunk.Idx, treasureIdxMap)
		}

		key := sqlWhereForGenesisHashAndChunkIdx(genesisHash, chunkIdx)
		chunksMap[key] = chunk
		minChunkIdx = oyster_utils.IntMin(minChunkIdx, chunkIdx)
		maxChunkIdx = oyster_utils.IntMax(maxChunkIdx, chunkIdx)
	}
	return chunksMap, minChunkIdx, maxChunkIdx
}

// convertToSQLKeyedMapForDataMap converts dataMaps into sql keyed maps. Remove any duplicate keyed.
func convertToSQLKeyedMapForDataMap(dataMaps []models.DataMap) map[string]models.DataMap {
	dmsMap := make(map[string]models.DataMap)
	for _, dm := range dataMaps {
		key := sqlWhereForGenesisHashAndChunkIdx(dm.GenesisHash, dm.ChunkIdx)
		// Only use the first one
		if _, hasKey := dmsMap[key]; !hasKey {
			dmsMap[key] = dm
		}
	}
	return dmsMap
}

func sqlWhereForGenesisHashAndChunkIdx(genesisHash string, chunkIdx int) string {
	return fmt.Sprintf("(genesis_hash = '%s' AND chunk_idx = %d)", genesisHash, chunkIdx)
}

func waitForTransferAndNotifyBeta(alphaEthAddr string, betaEthAddr string, uploadSessionId string) {

	if oyster_utils.BrokerMode != oyster_utils.ProdMode {
		return
	}

	transferAddr := services.StringToAddress(alphaEthAddr)
	balance, err := EthWrapper.WaitForTransfer(transferAddr, "prl")

	paymentStatus := models.PaymentStatusConfirmed
	if err != nil {
		paymentStatus = models.PaymentStatusError
	}

	session := models.UploadSession{}
	if err := models.DB.Find(&session, uploadSessionId); err != nil {
		oyster_utils.LogIfError(err, nil)
		return
	}

	if session.PaymentStatus != models.PaymentStatusConfirmed {
		session.PaymentStatus = paymentStatus
	}
	if err := models.DB.Save(&session); err != nil {
		oyster_utils.LogIfError(err, nil)
		return
	}

	// Alpha send half of it to Beta
	checkAndSendHalfPrlToBeta(session, balance)
}

func checkAndSendHalfPrlToBeta(session models.UploadSession, balance *big.Int) {
	if session.Type != models.SessionTypeAlpha ||
		session.PaymentStatus != models.PaymentStatusConfirmed ||
		session.ETHAddrBeta.String == "" {
		return
	}

	betaAddr := services.StringToAddress(session.ETHAddrBeta.String)
	betaBalance := EthWrapper.CheckPRLBalance(betaAddr)
	if betaBalance.Int64() > 0 {
		return
	}

	var splitAmount big.Int
	splitAmount.Set(balance)
	splitAmount.Div(balance, big.NewInt(2))
	callMsg := services.OysterCallMsg{
		From:   services.StringToAddress(session.ETHAddrAlpha.String),
		To:     betaAddr,
		Amount: splitAmount,
	}
	EthWrapper.SendPRL(callMsg)
}
