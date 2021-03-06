package actions

import (
	"github.com/oysterprotocol/brokernode/actions/utils"
	"github.com/oysterprotocol/brokernode/actions/v2"
	"github.com/oysterprotocol/brokernode/actions/v3"
	"github.com/oysterprotocol/brokernode/utils"

	"github.com/gobuffalo/buffalo"
	"github.com/prometheus/client_golang/prometheus"
)

var app *buffalo.App

// App is where all routes and middleware for buffalo
// should be defined. This is the nerve center of your
// application.
func App() *buffalo.App {
	if app == nil {
		app = actions_utils.CreateBuffaloApp()

		app.GET("/", HomeHandler)

		app.GET("/metrics", buffalo.WrapHandler(prometheus.Handler()))

		apiV2 := actions_v2.RegisterApi(app)

		// Status v2, will be deprecated (:3000/api/v2/status)
		statusResource := StatusResource{}
		apiV2.GET("status", statusResource.CheckStatus)

		// Status (:3000/status)
		app.GET("/status", statusResource.CheckStatus)

		actions_v3.RegisterApi(app)
	}

	oyster_utils.StartProfile()
	defer oyster_utils.StopProfile()

	return app
}
