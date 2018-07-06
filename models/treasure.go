package models

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"time"

	"github.com/gobuffalo/pop"
	"github.com/gobuffalo/uuid"
	"github.com/gobuffalo/validate"
	"github.com/oysterprotocol/brokernode/utils"
	"golang.org/x/crypto/sha3"
)

type PRLStatus int

// IMPORTANT:  Do not remove Message and Address from
// this struct; they are used for encryption
type Treasure struct {
	ID          uuid.UUID `json:"id" db:"id"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
	ETHAddr     string    `json:"ethAddr" db:"eth_addr"`
	ETHKey      string    `json:"ethKey" db:"eth_key"`
	PRLAmount   string    `json:"prlAmount" db:"prl_amount"`
	PRLStatus   PRLStatus `json:"prlStatus" db:"prl_status"`
	Message     string    `json:"message" db:"message"`
	MsgID       string    `json:"msgId" db:"msg_id"`
	Address     string    `json:"address" db:"address"`
	GenesisHash string    `json:"genesisHash" db:"genesis_hash"`
	PRLTxHash   string    `json:"prlTxHash" db:"prl_tx_hash"`
	PRLTxNonce  int64     `json:"prlTxNonce" db:"prl_tx_nonce"`
	GasTxHash   string    `json:"gasTxHash" db:"gas_tx_hash"`
	GasTxNonce  int64     `json:"gasTxNonce" db:"gas_tx_nonce"`
	BuryTxHash  string    `json:"buryTxHash" db:"bury_tx_hash"`
	BuryTxNonce int64     `json:"buryTxNonce" db:"bury_tx_nonce"`
}

const (
	// organizing these as a sequence to simplify queries and testing,
	// and because the process is a sequence anyway
	PRLWaiting PRLStatus = iota + 1
	PRLPending
	PRLConfirmed
	GasPending
	GasConfirmed
	BuryPending
	BuryConfirmed
	GasReclaimPending
	GasReclaimConfirmed

	// error states
	PRLError        = -1
	GasError        = -3
	BuryError       = -5
	GasReclaimError = -7
)

const maxNumSimultaneousTreasureTxs = 15

var PRLStatusMap = make(map[PRLStatus]string)

func init() {
	PRLStatusMap[PRLWaiting] = "PRLWaiting"
	PRLStatusMap[PRLPending] = "PRLPending"
	PRLStatusMap[PRLConfirmed] = "PRLConfirmed"
	PRLStatusMap[GasPending] = "GasPending"
	PRLStatusMap[GasConfirmed] = "GasConfirmed"
	PRLStatusMap[BuryPending] = "BuryPending"
	PRLStatusMap[BuryConfirmed] = "BuryConfirmed"
	PRLStatusMap[GasReclaimPending] = "GasReclaimPending"
	PRLStatusMap[GasReclaimConfirmed] = "GasReclaimConfirmed"

	PRLStatusMap[PRLError] = "PRLError"
	PRLStatusMap[GasError] = "GasError"
	PRLStatusMap[BuryError] = "BuryError"
}

// String is not required by pop and may be deleted
func (t Treasure) String() string {
	jt, _ := json.Marshal(t)
	return string(jt)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (t *Treasure) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (t *Treasure) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (t *Treasure) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

/**
 * Callbacks
 */

func (t *Treasure) BeforeCreate(tx *pop.Connection) error {

	// Defaults to PRLWaiting.
	if t.PRLStatus == 0 {
		t.PRLStatus = PRLWaiting
	}

	if len(t.Message) > 81 {
		// don't really need full message, just something known to
		// the broker that we can hash and then use for encryption
		t.Message = t.Message[0:81]
	}

	t.EncryptTreasureEthKey()

	return nil
}

func (t *Treasure) SetPRLAmount(bigInt *big.Int) (string, error) {
	prlAmountAsBytes, err := bigInt.MarshalJSON()
	if err != nil {
		oyster_utils.LogIfError(err, nil)
		return "", err
	}
	t.PRLAmount = string(prlAmountAsBytes)
	DB.ValidateAndUpdate(t)

	return t.PRLAmount, nil
}

func (t *Treasure) GetPRLAmount() *big.Int {

	prlAmountAsBytes := []byte(t.PRLAmount)
	var bigInt big.Int
	bigInt.UnmarshalJSON(prlAmountAsBytes)

	return &bigInt
}

func GetTreasuresToBuryByPRLStatus(prlStatuses []PRLStatus) ([]Treasure, error) {
	treasureRowsToReturn := make([]Treasure, 0)
	for _, prlStatus := range prlStatuses {
		treasureToBury := []Treasure{}
		err := DB.RawQuery("SELECT * from treasures where prl_status = ? LIMIT ?",
			prlStatus,
			maxNumSimultaneousTreasureTxs).All(&treasureToBury)
		if err != nil {
			oyster_utils.LogIfError(err, nil)
			return treasureToBury, err
		}
		treasureRowsToReturn = append(treasureRowsToReturn, treasureToBury...)
	}
	return treasureRowsToReturn, nil
}

func GetTreasuresToBuryByPRLStatusAndUpdateTime(prlStatuses []PRLStatus, thresholdTime time.Time) ([]Treasure, error) {
	timeSinceThreshold := time.Since(thresholdTime)

	treasureRowsToReturn := make([]Treasure, 0)

	for _, prlStatus := range prlStatuses {
		treasureToBury := []Treasure{}

		err := DB.RawQuery("SELECT * from treasures where prl_status = ? AND "+
			"TIMESTAMPDIFF(hour, updated_at, NOW()) >= ? LIMIT ?",
			prlStatus,
			int(timeSinceThreshold.Hours()),
			maxNumSimultaneousTreasureTxs).All(&treasureToBury)

		if err != nil {
			oyster_utils.LogIfError(err, nil)
			return treasureToBury, err
		}
		treasureRowsToReturn = append(treasureRowsToReturn, treasureToBury...)
	}
	return treasureRowsToReturn, nil
}

func GetAllTreasuresToBury() ([]Treasure, error) {
	allTreasures := []Treasure{}
	err := DB.RawQuery("SELECT * from treasures").All(&allTreasures)
	return allTreasures, err
}

func (t *Treasure) EncryptTreasureEthKey() {
	hashedMessage := oyster_utils.HashHex(hex.EncodeToString([]byte(t.Message)), sha3.New256())
	hashedAddress := oyster_utils.HashHex(hex.EncodeToString([]byte(t.Address)), sha3.New256())
	encryptedKey := oyster_utils.Encrypt(hashedMessage, t.ETHKey, hashedAddress)
	t.ETHKey = hex.EncodeToString(encryptedKey)
}

func (t *Treasure) DecryptTreasureEthKey() string {
	hashedMessage := oyster_utils.HashHex(hex.EncodeToString([]byte(t.Message)), sha3.New256())
	hashedAddress := oyster_utils.HashHex(hex.EncodeToString([]byte(t.Address)), sha3.New256())
	decryptedKey := oyster_utils.Decrypt(hashedMessage, t.ETHKey, hashedAddress)
	return hex.EncodeToString(decryptedKey)
}
