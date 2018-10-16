package actions

import (
	"os"

	raven "github.com/getsentry/raven-go"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/buffalo/middleware"
	"github.com/gobuffalo/buffalo/middleware/ssl"
	"github.com/gobuffalo/envy"
	"github.com/gobuffalo/x/sessions"
	"github.com/oysterprotocol/brokernode/actions/v2"
	"github.com/oysterprotocol/brokernode/actions/v3"
	"github.com/oysterprotocol/brokernode/jobs"
	"github.com/oysterprotocol/brokernode/models"
	"github.com/oysterprotocol/brokernode/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/cors"
	"github.com/unrolled/secure"
)

// ENV is used to help switch settings based on where the
// application is being run. Default is "development".
var ENV = envy.Get("GO_ENV", "development")
var app *buffalo.App

// App is where all routes and middleware for buffalo
// should be defined. This is the nerve center of your
// application.
func App() *buffalo.App {
	if app == nil {
		app = buffalo.New(buffalo.Options{
			Env:          ENV,
			LooseSlash:   true,
			SessionStore: sessions.Null{},
			PreWares: []buffalo.PreWare{
				cors.AllowAll().Handler,
			},
			SessionName: "_brokernode_session",
			WorkerOff:   false,
			Worker:      jobs.OysterWorker,
		})

		// Setup sentry
		ravenDSN := os.Getenv("SENTRY_DSN")
		if ravenDSN != "" {
			raven.SetDSN(ravenDSN)
		}

		// Automatically redirect to SSL
		app.Use(ssl.ForceSSL(secure.Options{
			SSLRedirect:     ENV == "production",
			SSLProxyHeaders: map[string]string{"X-Forwarded-Proto": "https"},
		}))

		// Set the request content type to JSON
		app.Use(middleware.SetContentType("application/json"))

		if ENV == "development" {
			app.Use(middleware.ParameterLogger)
		}

		// Wraps each request in a transaction.
		//  c.Value("tx").(*pop.PopTransaction)
		// Remove to disable this.
		app.Use(middleware.PopTransaction(models.DB))

		app.GET("/", HomeHandler)

		app.GET("/metrics", buffalo.WrapHandler(prometheus.Handler()))

		apiV2 := app.Group("/api/v2")
		apiV3 := app.Group("/api/v3")

		// UploadSessions
		uploadSessionResourceV2 := actions_v2.UploadSessionResourceV2{}
		apiV2.POST("upload-sessions", uploadSessionResourceV2.Create)
		apiV2.PUT("upload-sessions/{id}", uploadSessionResourceV2.Update)
		apiV2.POST("upload-sessions/beta", uploadSessionResourceV2.CreateBeta)
		apiV2.GET("upload-sessions/{id}", uploadSessionResourceV2.GetPaymentStatus)

		uploadSessionResourceV3 := actions_v3.UploadSessionResourceV3{}
		apiV3.POST("upload-sessions", uploadSessionResourceV2.Create)
		apiV3.PUT("upload-sessions/{id}", uploadSessionResourceV3.Update)
		apiV3.POST("upload-sessions/beta", uploadSessionResourceV2.CreateBeta)
		apiV3.GET("upload-sessions/{id}", uploadSessionResourceV2.GetPaymentStatus)

		// Webnodes
		webnodeResource := actions_v2.WebnodeResource{}
		apiV2.POST("supply/webnodes", webnodeResource.Create)

		// Transactions
		transactionBrokernodeResource := actions_v2.TransactionBrokernodeResource{}
		apiV2.POST("demand/transactions/brokernodes", transactionBrokernodeResource.Create)
		apiV2.PUT("demand/transactions/brokernodes/{id}", transactionBrokernodeResource.Update)

		transactionGenesisHashResource := actions_v2.TransactionGenesisHashResource{}
		apiV2.POST("demand/transactions/genesis_hashes", transactionGenesisHashResource.Create)
		apiV2.PUT("demand/transactions/genesis_hashes/{id}", transactionGenesisHashResource.Update)

		// Treasure claims
		treasures := actions_v2.TreasuresResource{}
		apiV2.POST("treasures", treasures.VerifyAndClaim)

		// Status
		statusResource := actions_v2.StatusResource{}
		apiV2.GET("status", statusResource.CheckStatus)

		// Treasure signing
		signTreasureResource := actions_v2.SignTreasureResource{}
		apiV2.GET("unsigned-treasure/{id}", signTreasureResource.GetUnsignedTreasure)
		apiV2.PUT("signed-treasure/{id}", signTreasureResource.SignTreasure)
	}

	oyster_utils.StartProfile()
	defer oyster_utils.StopProfile()

	return app
}
