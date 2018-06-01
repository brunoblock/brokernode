package actions

import (
	"github.com/gobuffalo/buffalo"
	"github.com/oysterprotocol/brokernode/models"
	"github.com/oysterprotocol/brokernode/services"
	"github.com/oysterprotocol/brokernode/utils"
	"time"
)

type TreasuresResource struct {
	buffalo.Resource
}

type treasureReq struct {
	ReceiverEthAddr string `json:"receiverEthAddr"`
	GenesisHash     string `json:"genesisHash"`
	SectorIdx       int    `json:"sectorIdx"`
	NumChunks       int    `json:"numChunks"`
	EthKey          string `json:"ethKey"`
}

type treasureRes struct {
	Success bool `json:"success"`
}

// Verifies the treasure and claims such treasure.
func (t *TreasuresResource) VerifyAndClaim(c buffalo.Context) error {

	startTime := time.Now()

	defer func() {
		PrometheusWrapper.HistogramSeconds(HistogramTreasuresVerifyAndClaim, startTime)
	}()

	req := treasureReq{}
	oyster_utils.ParseReqBody(c.Request(), &req)

	addr := models.ComputeSectorDataMapAddress(req.GenesisHash, req.SectorIdx, req.NumChunks)
	verify, err := IotaWrapper.VerifyTreasure(addr)

	if err == nil && verify {
		ethAddr := EthWrapper.GenerateEthAddrFromPrivateKey(req.EthKey)
		verify = EthWrapper.ClaimPRL(services.StringToAddress(req.ReceiverEthAddr), ethAddr, req.EthKey)
	}

	res := treasureRes{
		Success: verify,
	}

	return c.Render(200, r.JSON(res))
}
