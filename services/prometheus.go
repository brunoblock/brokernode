package services

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	PrometheusWrapper PrometheusService
)

type PrepareHistogram func(name string, help string, labelNames ...string) (histogram *prometheus.HistogramVec)
type HistogramSeconds func(histogram *prometheus.HistogramVec, start time.Time) ()
type HistogramData func(histogram *prometheus.HistogramVec, data float64) ()
type TimeNow func() (start time.Time)

type PrometheusService struct {
	PrepareHistogram
	HistogramSeconds
	HistogramData
	TimeNow
	HistogramTreasuresVerifyAndClaim *prometheus.HistogramVec
	HistogramUploadSessionResourceCreate *prometheus.HistogramVec
	HistogramUploadSessionResourceUpdate *prometheus.HistogramVec
	HistogramUploadSessionResourceCreateBeta *prometheus.HistogramVec
	HistogramUploadSessionResourceGetPaymentStatus *prometheus.HistogramVec
	HistogramWebnodeResourceCreate *prometheus.HistogramVec
	HistogramTransactionBrokernodeResourceCreate *prometheus.HistogramVec
	HistogramTransactionBrokernodeResourceUpdate *prometheus.HistogramVec
	HistogramTransactionGenesisHashResourceCreate *prometheus.HistogramVec
	HistogramTransactionGenesisHashResourceUpdate *prometheus.HistogramVec
	HistogramClaimUnusedPRLs *prometheus.HistogramVec
	HistogramFlushOldWebNodes *prometheus.HistogramVec
	HistogramProcessPaidSessions *prometheus.HistogramVec
	HistogramProcessUnassignedChunks *prometheus.HistogramVec
	HistogramPurgeCompletedSessions *prometheus.HistogramVec
	HistogramRemoveUnpaidUploadSession *prometheus.HistogramVec
	HistogramUpdateTimeOutDataMaps *prometheus.HistogramVec
	HistogramVerifyDataMaps *prometheus.HistogramVec
}

func init() {
	histogramTreasuresVerifyAndClaim := prepareHistogram("treasures_verify_and_claim_seconds", "HistogramTreasuresVerifyAndClaimSeconds", "code")
	histogramUploadSessionResourceCreate := prepareHistogram("upload_session_resource_create_seconds", "HistogramUploadSessionResourceCreateSeconds", "code")
	histogramUploadSessionResourceUpdate := prepareHistogram("upload_session_resource_update_seconds", "HistogramUploadSessionResourceUpdateSeconds", "code")
	histogramUploadSessionResourceCreateBeta := prepareHistogram("upload_session_resource_create_beta_seconds", "HistogramUploadSessionResourceCreateBetaSeconds", "code")
	histogramUploadSessionResourceGetPaymentStatus := prepareHistogram("upload_session_resource_get_payment_status_seconds", "HistogramUploadSessionResourceGetPaymentStatusSeconds", "code")
	histogramWebnodeResourceCreate := prepareHistogram("webnode_resource_create_seconds", "HistogramWebnodeResourceCreateSeconds", "code")
	histogramTransactionBrokernodeResourceCreate := prepareHistogram("transaction_brokernode_resource_create_seconds", "HistogramTransactionBrokernodeResourceCreateSeconds", "code")
	histogramTransactionBrokernodeResourceUpdate := prepareHistogram("transaction_brokernode_resource_update_seconds", "HistogramTransactionBrokernodeResourceUpdateSeconds", "code")
	histogramTransactionGenesisHashResourceCreate := prepareHistogram("transaction_genesis_hash_resource_create_seconds", "HistogramTransactionGenesisHashResourceCreateSeconds", "code")
	histogramTransactionGenesisHashResourceUpdate := prepareHistogram("transaction_genesis_hash_resource_seconds", "HistogramTransactionGenesisHashResourceUpdateSeconds", "code")
	histogramClaimUnusedPRLs := prepareHistogram("claim_unsed_prls_seconds", "HistogramClaimUnusedPRLsSeconds", "code")
	histogramFlushOldWebNodes := prepareHistogram("flush_old_web_nodes_seconds", "HistogramFlushOldWebNodes", "code")
	histogramProcessPaidSessions := prepareHistogram("process_paid_sessions_seconds", "HistogramProcessPaidSessions", "code")
	histogramProcessUnassignedChunks := prepareHistogram("process_unassigned_chunks_seconds", "HistogramProcessUnassignedChunks", "code")
	histogramPurgeCompletedSessions := prepareHistogram("purge_completed_sessions_seconds", "HistogramPurgeCompletedSessions", "code")
	histogramRemoveUnpaidUploadSession := prepareHistogram("remove_unpaid_upload_session_seconds", "HistogramRemoveUnpaidUploadSession", "code")
	histogramUpdateTimeOutDataMaps := prepareHistogram("update_time_out_datamaps_seconds", "HistogramUpdateTimeOutDataMaps", "code")
	histogramVerifyDataMaps := prepareHistogram("verify_datamaps_seconds", "HistogramVerifyDataMaps", "code")

	PrometheusWrapper = PrometheusService{
		PrepareHistogram: prepareHistogram,
		HistogramSeconds: histogramSeconds,
		HistogramData: histogramData,
		TimeNow: timeNow,
		HistogramTreasuresVerifyAndClaim: histogramTreasuresVerifyAndClaim,
		HistogramUploadSessionResourceCreate: histogramUploadSessionResourceCreate,
		HistogramUploadSessionResourceUpdate: histogramUploadSessionResourceUpdate,
		HistogramUploadSessionResourceCreateBeta: histogramUploadSessionResourceCreateBeta,
		HistogramUploadSessionResourceGetPaymentStatus: histogramUploadSessionResourceGetPaymentStatus,
		HistogramWebnodeResourceCreate: histogramWebnodeResourceCreate,
		HistogramTransactionBrokernodeResourceCreate: histogramTransactionBrokernodeResourceCreate,
		HistogramTransactionBrokernodeResourceUpdate: histogramTransactionBrokernodeResourceUpdate,
		HistogramTransactionGenesisHashResourceCreate: histogramTransactionGenesisHashResourceCreate,
		HistogramTransactionGenesisHashResourceUpdate: histogramTransactionGenesisHashResourceUpdate,
		HistogramClaimUnusedPRLs: histogramClaimUnusedPRLs,
		HistogramFlushOldWebNodes: histogramFlushOldWebNodes,
		HistogramProcessPaidSessions: histogramProcessPaidSessions,
		HistogramProcessUnassignedChunks: histogramProcessUnassignedChunks,
		HistogramPurgeCompletedSessions: histogramPurgeCompletedSessions,
		HistogramRemoveUnpaidUploadSession: histogramRemoveUnpaidUploadSession,
		HistogramUpdateTimeOutDataMaps: histogramUpdateTimeOutDataMaps,
		HistogramVerifyDataMaps: histogramVerifyDataMaps,
	}
}

func duration(start time.Time) (duration time.Duration) {
	duration = time.Since(start)
	return duration
}

func timeNow() (start time.Time) {
	start = time.Now()
	return start
}

func prepareHistogram(name string, help string, labelNames ...string) (histogram *prometheus.HistogramVec) {
	histogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: name,
		Help: help,
	}, labelNames)

	prometheus.Register(histogram)
	return histogram
}

func histogramSeconds(histogram *prometheus.HistogramVec, start time.Time) () {
	duration := duration(start)
	histogram.WithLabelValues("500").Observe(duration.Seconds())
}

func histogramData(histogram *prometheus.HistogramVec, data float64) () {
	histogram.WithLabelValues("500").Observe(data)
}
