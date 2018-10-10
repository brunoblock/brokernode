package actions_v2

import (
	"testing"

	"github.com/gobuffalo/suite"
	"github.com/oysterprotocol/brokernode/actions/utils"
	"github.com/oysterprotocol/brokernode/services"
	"github.com/oysterprotocol/brokernode/utils"
)

type ActionSuite struct {
	*suite.Action
}

func (suite *ActionSuite) SetupTest() {
	suite.Action.SetupTest()

	suite.Nil(oyster_utils.InitKvStore())

	EthWrapper = services.EthWrapper
	IotaWrapper = services.IotaWrapper
}

func Test_ActionSuite(t *testing.T) {
	oyster_utils.SetBrokerMode(oyster_utils.ProdMode)
	defer oyster_utils.ResetBrokerMode()
	app := actions_utils.CreateBuffaloApp()
	RegisterApi(app)
	as := &ActionSuite{suite.NewAction(app)}
	suite.Run(t, as)
}
