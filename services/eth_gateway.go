package services

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/getsentry/raven-go"
	"github.com/joho/godotenv"
	"github.com/oysterprotocol/brokernode/models"
	"log"
	"os"
	"sync"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
	"crypto/ecdsa"
	"errors"
	"time"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"strings"
	"github.com/ethereum/go-ethereum/core/vm"
)

type Eth struct {
	SendGas             SendGas
	ClaimUnusedPRLs     ClaimPRLs
	GenerateEthAddr     GenerateEthAddr
	BuryPrl             BuryPrl
	SendETH             SendETH
	SendPRL             SendPRL
	GetGasPrice         GetGasPrice
	SubscribeToTransfer SubscribeToTransfer
	CheckBalance        CheckBalance
	GetCurrentBlock     GetCurrentBlock
	OysterCallMsg	    OysterCallMsg
}

type OysterCallMsg struct {
	From common.Address
	To common.Address
	Amount big.Int
	PrivateKey ecdsa.PrivateKey
	Gas uint64
	GasPrice big.Int
	TotalWei big.Int
	Data []byte
}

type SendGas func([]models.CompletedUpload) error
type GenerateEthAddr func() (addr common.Address, privateKey string, err error)
type GetGasPrice func() (*big.Int, error)
type SubscribeToTransfer func(brokerAddr common.Address, outCh chan<- types.Log)
type CheckBalance func(common.Address) (*big.Int)
type GetCurrentBlock func() (*types.Block, error)
type SendETH func(toAddr common.Address, amount *big.Int) (rawTransaction string)

type BuryPrl func(msg OysterCallMsg) (bool)
type SendPRL func(msg OysterCallMsg) (bool)
type ClaimPRLs func([]models.CompletedUpload) error

// Singleton client
var (
	ethUrl            string
	MainWalletAddress common.Address
	MainWalletKey     string
	client            *ethclient.Client
	mtx               sync.Mutex
	EthWrapper        Eth
)

func init() {
	// Load ENV variables
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
		raven.CaptureError(err, nil)
	}

	MainWalletAddress := os.Getenv("MAIN_WALLET_ADDRESS")
	MainWalletKey := os.Getenv("MAIN_WALLET_KEY")
	ethUrl := os.Getenv("ETH_NODE_URL")

	fmt.Println(MainWalletAddress)
	fmt.Println(MainWalletKey)
	fmt.Println(ethUrl)

	EthWrapper = Eth{
		SendGas:         sendGas,
		ClaimUnusedPRLs: claimPRLs,
		GenerateEthAddr: generateEthAddr,
		BuryPrl:         buryPrl,
		SendETH:         sendETH,
		SendPRL:         sendPRL,
		GetGasPrice:     getGasPrice,
		SubscribeToTransfer: subscribeToTransfer,
		CheckBalance:        checkBalance,
		GetCurrentBlock:     getCurrentBlock,
	}
}

// Shared client provides access to the underlying Ethereum client
func sharedClient(netUrl string) (c *ethclient.Client, err error) {
	if client != nil {
		return client, nil
	}
	// check-lock-check pattern to avoid excessive locking.
	mtx.Lock()
	defer mtx.Unlock()

	if client != nil {
		// override to allow custom node url
		if len(netUrl)>0 {
			ethUrl = netUrl
		}
		c, err = ethclient.Dial(ethUrl)
		if err != nil {
			fmt.Println("Failed to dial in to Ethereum node.")
			return
		}
		// Sets Singleton
		client = c
	}
	return client, err
}

// Generate an Ethereum address
func generateEthAddr() (addr common.Address, privateKey string, err error) {
	ethAccount, _ := crypto.GenerateKey()
	addr = crypto.PubkeyToAddress(ethAccount.PublicKey)
	privateKey = hex.EncodeToString(ethAccount.D.Bytes())
	return addr, privateKey, err
}

// returns represents the 20 byte address of an ethereum account.
func stringToAddress(address string) common.Address {
	return common.HexToAddress(address)
}

// SuggestGasPrice retrieves the currently suggested gas price to allow a timely
// execution for new transaction
func getGasPrice() (*big.Int, error) {
	// connect ethereum client
	client, err := sharedClient("")
	if err != nil {
		log.Fatal("Could not get gas price from network")
	}
	
  // there is no guarantee with estimate gas price
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		log.Fatal("Client could not get gas price from network")
	}
	return gasPrice, nil
  
  // TODO review this doesn't return gasPrice
	// addr = crypto.PubkeyToAddress(ethAccount.PublicKey).Hex()
	// privKey = hex.EncodeToString(ethAccount.D.Bytes())
	//oyster_utils.LogToSegment("eth_gateway: generated_new_eth_address", analytics.NewProperties().
		//Set("eth_address", fmt.Sprint(addr)))
	//return

}

// Check balance from a valid Ethereum network address
func checkBalance(addr common.Address) (*big.Int) {
	// connect ethereum client
	client, err := sharedClient("")
	if err != nil {
		log.Fatal("Could not initialize shared client")
	}

	balance, err := client.BalanceAt(context.Background(),addr, nil)  //Call(&bal, "eth_getBalance", addr, "latest")
	if err != nil {
		fmt.Println("Client could not retrieve balance:", err)
		return big.NewInt(0)
	}
	return balance
}

// Get current block from blockchain
func getCurrentBlock() (*types.Block, error) {
	// connect ethereum client
	client, err := sharedClient("")
	if err != nil {
		log.Fatal("Could not connect to Ethereum network", err)
		return nil, err
	}

	// latest block number is nil to get the latest block
	currentBlock, err := client.BlockByNumber(context.Background(), nil)
	if err != nil {
		fmt.Printf("Could not get last block: %v\n", err)
		return nil, err
	}

	// latest block event
	fmt.Printf("latest block: %v\n", currentBlock.Number())
	return currentBlock, nil
}

// SubscribeToTransfer will subscribe to transfer events
// sending PRL to the brokerAddr given.
// Notifications will be sent in the out channel provided.
func subscribeToTransfer(brokerAddr common.Address, outCh chan<- types.Log) {
	client, _ := sharedClient("")
	currentBlock, _ := getCurrentBlock()
	q := ethereum.FilterQuery{
		FromBlock: currentBlock.Number(), // beginning of the queried range, nil means genesis block
		ToBlock: nil, // end of the range, nil means latest block
		Addresses: []common.Address{brokerAddr},
		Topics:  nil, // matches any topic list
	}

	// subscribe before passing it to outCh.
	sub, _ := client.SubscribeFilterLogs(context.Background(), q, outCh)

	for {
		select {
		case err := <-sub.Err():
			log.Fatal(err)
		/*
		case log := <- outCh:
			fmt.Printf("Log Data:%v", log.Data)

			// need to add unpack abi result if
			// the method call is for the contract
			if err != nil {
				fmt.Println("Failed to unpack:", err)
			}

			fmt.Println("Confirmed Address:", log.Address.Hex())

			sub.Unsubscribe()

			// TODO ensure confirmation type from "sendGas" or "sendPRL"
			recordTransaction(log.Address, "")*/
		}
	}
}

// Send gas to the completed upload Ethereum account
func sendGas(completedUploads []models.CompletedUpload) (error) {
	for _, completedUpload := range completedUploads {
		// returns a raw transaction, we may need to store them to verify all transactions are completed
		// mock value need to get amount, not in completed upload object
		gasPrice, _ := getGasPrice()
		sendETH(stringToAddress(completedUpload.ETHAddr), gasPrice)
	}
	return nil
}

// Transfer funds from one Ethereum account to another.
// We need to pass in the credentials, to allow the transaction to execute.
func sendETH(toAddr common.Address, amount *big.Int) (rawTransaction string) {

	client, _ := sharedClient("")
	// initialize the context
	deadline := time.Now().Add(1000 * time.Millisecond)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	// generate nonce
	nonce, _ := client.NonceAt(ctx, MainWalletAddress, nil)

	// get latest gas limit & price - current default gasLimit on oysterby 21000
	gasLimit := uint64(21000) // may pull gas limit from estimate gas price
	gasPrice, _ := getGasPrice()

	// create new transaction
	tx := types.NewTransaction(nonce, toAddr, amount, gasLimit, gasPrice, nil)

	// oysterby chainId 559966
	chainId := big.NewInt(559966)
	privateKey, _ := crypto.HexToECDSA(MainWalletKey)
	signer := types.NewEIP155Signer(chainId)
	signedTx, err := types.SignTx(tx, signer, privateKey)

	// send transaction
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return err.Error()
	}

	// pull signed transaction
	ts := types.Transactions{signedTx}
	// return raw transaction
	rawTransaction = string(ts.GetRlp(0))

	return
}

// Bury PRLs
func buryPrl(msg OysterCallMsg) (bool) {

	// dispense PRLs from the transaction address to each 'treasure' address
	rawTransaction := sendETH(msg.To, &msg.Amount)
  
	if len(rawTransaction) < 0 {
		// sending eth has failed
		return false
	}

	// initialize the context
	ctx, cancel := createContext()
	defer cancel()
	// shared client
	client, _ := sharedClient("")
	// abi
	oysterABI, err := abi.JSON(strings.NewReader(OysterPearlABI))
	// oyster contract method bury() no args
	buryPRL, _ := oysterABI.Pack("bury")
	// build transaction and sign
	signedTx, err := callOysterPearl(ctx, buryPRL)
	// send transaction
	err = client.SendTransaction(ctx, signedTx)

	if err != nil {
		return false
	}
	// pull signed transaction
	ts := types.Transactions{signedTx}
	// return raw transaction
	rawTransaction = string(ts.GetRlp(0))

	// successful contract message call
	if len(rawTransaction) > 0 {
		return true
	} else {
		return false
	}
}

// Claim PRLs from OysterPearl contract
func claimPRLs(completedUploads []models.CompletedUpload) error {

	// Contract claim(address _payout, address _fee) public returns (bool success)
	for _, completedUpload := range completedUploads {
		//1
		//	    for each completed upload, get its PRL balance from its ETH
		//		address (completedUpload.ETHAddr) by calling CheckBalance.
		var ethAddr = stringToAddress(completedUpload.ETHAddr)
		var balance = checkBalance(ethAddr)
		if balance.Int64() <= 0 {
			// need to log this error to apply a retry
			return errors.New("could not complete transaction due to zero balance for:"+completedUpload.ETHAddr)
		}
		//2.
		//	    Then, using SendPRL, create a transaction with each
		//	    completedUpload.ETHAddr as the "fromAddr" address, the broker's
		//	    main wallet (MainWalletAddress) as the "toAddr" address,
		var from = ethAddr
		var to = MainWalletAddress
		//3.
		// 		and the PRL balance of completedUpload.ETHAddr as the "amt" to send,
		// 		and subscribe to the event with SubscribeToTransfer.
		var amountToSend = balance
		var gas = uint64(21000)           // TODO get gas source are we pulling from ETHAddr?
		gasPrice, _ := getGasPrice()

		// prepare oyster message call
		var oysterMsg = OysterCallMsg{
			From: from,
			To: to,
			Amount: *amountToSend,
			Gas: gas,					 // TODO gas
			GasPrice: *gasPrice,         // TODO ensure gas price is valid and sufficient
			TotalWei: *big.NewInt(1), // TODO finish wei
			Data: []byte(""), // setup data
		}
		// send transaction from completed upload eth addr to main wallet
		if !sendPRL(oysterMsg) {
			// TODO more detailed error message
			return errors.New("unable to send prl to main wallet")
		}
	}

	return nil
}

/*
	When a user uploads a file, we create an upload session on the broker.

	For each "upload session", we generate a new wallet (we do this so we can associate a session to PRLs sent).
	The broker responds to the uploader with an invoice to send X PRLs to Y eth address
	The broker then listens for a transfer event so it knows when payment has happened,

	Once the payment is received, the brokers will split up the PRLs and begin work to attach the file to the IOTA tangle
	so to answer your question, there won't be a main PRL wallet, the address is different for each session
	however there will be a "main" ETH wallet, which is used to pay gas fees
 */
func sendPRL(msg OysterCallMsg) (bool) {

	// initialize the context
	ctx, cancel := createContext()
	defer cancel()

	// shared client
	client, _ := sharedClient("")
	// abi
	oysterABI, err := abi.JSON(strings.NewReader(OysterPearlABI))
	// oyster contract method transfer(address _to, uint256 _value)
	sendPRL, _ := oysterABI.Pack("transfer", msg.To.Hex(), msg.Amount)
	// build transaction and sign
	signedTx, err := callOysterPearl(ctx, sendPRL)
	// send transaction
	err = client.SendTransaction(ctx, signedTx)

	if err != nil {
		return false
	}
	// pull signed transaction
	ts := types.Transactions{signedTx}
	// return raw transaction
	rawTransaction := string(ts.GetRlp(0))

	// successful contract message call
	if len(rawTransaction) > 0 {
		// TODO pull stash for subscribe to transfer
		return true
	} else {
		return false
	}
}

// utility to call a method on OysterPearl contract
func callOysterPearl(ctx context.Context, data []byte) (*types.Transaction, error) {

	// invoke the smart contract bury() function with 'treasure'
	// TODO OysterPearl
	contractAddress := common.HexToAddress("0xf25186b5081ff5ce73482ad761db0eb0d25abfbf")

	// oysterby chainId 559966 - env
	chainId := big.NewInt(559966)
	privateKey, err := crypto.HexToECDSA(MainWalletKey)
	if err != nil {
		fmt.Printf("Failed to parse secp256k1 private key")
		return nil, err
	}
	client, _ := sharedClient("")
	nonce, _ := client.NonceAt(ctx, MainWalletAddress, nil)

	// get latest gas limit & price - current default gasLimit on oysterby 21000
	gasLimit := uint64(vm.GASLIMIT) // may pull gas limit from estimate gas price
	gasPrice, _ := getGasPrice()

	// create new transaction with 0 amount
	tx := types.NewTransaction(nonce, contractAddress, big.NewInt(0), gasLimit, gasPrice, data)

	signer := types.NewEIP155Signer(chainId)
	signedTx, _ := types.SignTx(tx, signer, privateKey)

	return  signedTx, nil
}

// context helper to include the deadline initialization
func createContext() (ctx context.Context, cancel context.CancelFunc) {
	deadline := time.Now().Add(1000 * time.Millisecond)
	return context.WithDeadline(context.Background(), deadline)
}


// TODO will be Use channels/workers for subscribe to transaction events
// There is an example of a channel/worker in iota_wrappers.go
// These methods live in models/completed_uploads.go
func recordTransaction(address common.Address, status string) {
	// when a successful transaction event, will need to change the status
	// of the correct row in the completed_uploads table.

	// expect "address" to be the "to" address of the gas transaction or
	// the "from" address of the PRL transaction.

	// do *not* use the broker's main wallet address
	switch status {
	case "sendGas":
		// gas transfers succeeded, call this method:
		models.SetGasStatusByAddress(address.Hex(), models.GasTransferSuccess)
	case "sendPRL":
		// PRL transfer succeeded, call this:
		models.SetPRLStatusByAddress(address.Hex(), models.PRLClaimSuccess)
	}
}
