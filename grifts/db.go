package grifts

import (
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/markbates/grift/grift"
	"github.com/oysterprotocol/brokernode/models"
	"github.com/oysterprotocol/brokernode/services"
	"github.com/oysterprotocol/brokernode/utils"
	"math/big"
	"os"
	"strconv"
	"time"
)

const qaTrytes = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
const qaGenHashStartingChars = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

func getAddress() (common.Address, string, error) {
	griftPrivateKey := os.Getenv("GRIFT_ETH_PRIVATE_KEY")
	if griftPrivateKey == "" {
		errorString := "you haven't specified an eth private key to use for this grift"
		fmt.Println(errorString)
		return services.StringToAddress(""), griftPrivateKey, errors.New(errorString)
	}
	address := services.EthWrapper.GenerateEthAddrFromPrivateKey(griftPrivateKey)
	return address, griftPrivateKey, nil
}

var _ = grift.Namespace("db", func() {

	grift.Desc("seed", "Seeds a database")
	grift.Add("seed", func(c *grift.Context) error {
		// Add DB seeding stuff here
		return nil
	})

	grift.Desc("send_prl_seed", "Adds a 'treasure' that needs PRL")
	grift.Add("send_prl_seed", func(c *grift.Context) error {

		var numToCreate int
		if len(c.Args) == 0 {
			numToCreate = 1
		} else {
			numToCreate, _ = strconv.Atoi(c.Args[0])
		}

		for i := 0; i < numToCreate; i++ {
			address, griftPrivateKey, err := services.EthWrapper.GenerateEthAddr()
			fmt.Println("PRIVATE KEY IS:")
			fmt.Println(griftPrivateKey)

			if err != nil {
				fmt.Println(err)
				return err
			}

			//prlAmount := big.NewFloat(float64(.0001))
			//prlAmountInWei := oyster_utils.ConvertToWeiUnit(prlAmount)
			prlAmountInWei := big.NewInt(7800000000000001)

			treasure := models.Treasure{
				ETHAddr: address.Hex(),
				ETHKey:  griftPrivateKey,
				Address: qaTrytes,
				Message: qaTrytes,
			}

			treasure.SetPRLAmount(prlAmountInWei)

			vErr, err := models.DB.ValidateAndCreate(&treasure)

			if err == nil && len(vErr.Errors) == 0 {
				fmt.Println("Treasure row added")
			}
		}

		return nil
	})

	grift.Desc("send_prl_remove", "Removes the 'treasure' that needs PRL")
	grift.Add("send_prl_remove", func(c *grift.Context) error {

		err := models.DB.RawQuery("DELETE from treasures WHERE address = ?", qaTrytes).All(&[]models.Treasure{})

		if err == nil {
			fmt.Println("Treasure row deleted")
		}

		return nil
	})

	grift.Desc("set_to_prl_waiting", "Stages treasure for PRL")
	grift.Add("set_to_prl_waiting", func(c *grift.Context) error {

		address, _, err := getAddress()
		if err != nil {
			fmt.Println(err)
			return err
		}

		treasureToBury := models.Treasure{}

		err = models.DB.RawQuery("SELECT * from treasures where eth_addr = ?", address.Hex()).First(&treasureToBury)

		if err == nil {
			fmt.Println("Found transaction!")
		}

		treasureToBury.PRLStatus = models.PRLWaiting

		vErr, err := models.DB.ValidateAndUpdate(&treasureToBury)
		if err == nil && len(vErr.Errors) == 0 {
			fmt.Println("Updated!")
		}

		return nil
	})

	grift.Desc("set_to_prl_confirmed", "Stages treasure for gas")
	grift.Add("set_to_prl_confirmed", func(c *grift.Context) error {

		address, _, err := getAddress()
		if err != nil {
			fmt.Println(err)
			return err
		}

		treasureToBury := models.Treasure{}

		err = models.DB.RawQuery("SELECT * from treasures where eth_addr = ?", address.Hex()).First(&treasureToBury)

		if err == nil {
			fmt.Println("Found transaction!")
		}

		treasureToBury.PRLStatus = models.PRLConfirmed

		vErr, err := models.DB.ValidateAndUpdate(&treasureToBury)
		if err == nil && len(vErr.Errors) == 0 {
			fmt.Println("Updated!")
		}

		return nil
	})

	grift.Desc("set_to_gas_confirmed", "Stages treasure for bury()")
	grift.Add("set_to_gas_confirmed", func(c *grift.Context) error {

		address, _, err := getAddress()
		if err != nil {
			fmt.Println(err)
			return err
		}

		treasureToBury := models.Treasure{}

		err = models.DB.RawQuery("SELECT * from treasures where eth_addr = ?", address.Hex()).First(&treasureToBury)

		if err == nil {
			fmt.Println("Found transaction!")
		}

		treasureToBury.PRLStatus = models.PRLConfirmed

		vErr, err := models.DB.ValidateAndUpdate(&treasureToBury)
		if err == nil && len(vErr.Errors) == 0 {
			fmt.Println("Updated!")
		}

		return nil
	})

	grift.Desc("print_treasure", "Prints the treasure you are testing with")
	grift.Add("print_treasure", func(c *grift.Context) error {

		treasuresToBury := []models.Treasure{}

		err := models.DB.RawQuery("SELECT * from treasures where address = ?", qaTrytes).All(&treasuresToBury)

		if err == nil {
			for _, treasureToBury := range treasuresToBury {
				fmt.Println("ETH Address:  " + treasureToBury.ETHAddr)
				fmt.Println("ETH Key:      " + treasureToBury.ETHKey)
				fmt.Println("Iota Address: " + treasureToBury.Address)
				fmt.Println("Iota Message: " + treasureToBury.Message)
				fmt.Println("PRL Status:   " + models.PRLStatusMap[treasureToBury.PRLStatus])
				fmt.Println("PRL Amount:   " + treasureToBury.PRLAmount)
			}
		} else {
			fmt.Println(err)
		}

		return nil
	})

	grift.Desc("delete_uploads", "Removes any sessions or data_maps in the db")
	grift.Add("delete_uploads", func(c *grift.Context) error {

		models.DB.RawQuery("DELETE from upload_sessions").All(&[]models.UploadSession{})
		models.DB.RawQuery("DELETE from data_maps").All(&[]models.DataMap{})

		// Clean up KvStore
		services.RemoveAllKvStoreData()
		services.InitKvStore()
		return nil
	})

	grift.Desc("delete_genesis_hashes", "Delete all stored genesis hashes")
	grift.Add("delete_genesis_hashes", func(c *grift.Context) error {

		models.DB.RawQuery("DELETE from stored_genesis_hashes").All(&[]models.StoredGenesisHash{})

		return nil
	})

	grift.Desc("reset_genesis_hashes", "Resets all stored genesis hashes to webnode count 0 and status unassigned")
	grift.Add("reset_genesis_hashes", func(c *grift.Context) error {

		storedGenHashCount := models.StoredGenesisHash{}

		count, err := models.DB.RawQuery("SELECT COUNT(*) from stored_genesis_hashes").Count(&storedGenHashCount)

		if count == 0 {
			fmt.Println("No stored genesis hashes available!")
			return nil
		}

		err = models.DB.RawQuery("UPDATE stored_genesis_hashes SET webnode_count = ? AND status = ?",
			0, models.StoredGenesisHashUnassigned).All(&[]models.StoredGenesisHash{})

		if err == nil {
			fmt.Println("Successfully reset all stored genesis hashes!")
		} else {
			fmt.Println(err)
			return err
		}

		return nil
	})

	grift.Desc("add_brokernodes", "add some brokernode addresses to the db")
	grift.Add("add_brokernodes", func(c *grift.Context) error {

		qaBrokerIPs := []string{
			"52.14.218.135", "18.217.133.146",
		}

		hostIP := os.Getenv("HOST_IP")

		for _, qaBrokerIP := range qaBrokerIPs {
			if qaBrokerIP != hostIP {
				vErr, err := models.DB.ValidateAndCreate(&models.Brokernode{
					Address: "http://" + qaBrokerIP + ":3000",
				})
				if err != nil || len(vErr.Errors) != 0 {
					fmt.Println(err)
					fmt.Println(vErr)
					return err
				}
			}
		}

		fmt.Println("Successfully added brokernodes to database!")

		return nil
	})

	grift.Desc("delete_brokernodes", "delete all brokernode addresses from the db")
	grift.Add("delete_brokernodes", func(c *grift.Context) error {

		err := models.DB.RawQuery("DELETE from brokernodes").All(&[]models.Brokernode{})

		if err != nil {
			fmt.Println(err)
			return err
		}

		fmt.Println("Successfully deleted brokernodes from database!")
		return nil
	})

	grift.Desc("claim_unused_test", "Adds a completed upload "+
		"to claim unused PRLs from")
	grift.Add("claim_unused_test", func(c *grift.Context) error {

		var numToCreate int
		if len(c.Args) == 0 {
			numToCreate = 1
		} else {
			numToCreate, _ = strconv.Atoi(c.Args[0])
		}

		for i := 0; i < numToCreate; i++ {
			address, griftPrivateKey, err := services.EthWrapper.GenerateEthAddr()
			fmt.Println("PRIVATE KEY IS:")
			fmt.Println(griftPrivateKey)

			if err != nil {
				fmt.Println(err)
				return err
			}

			//prlAmountInWei := big.NewInt(7800000000000001)

			prlAmount := big.NewFloat(float64(.0001))
			prlAmountInWei := oyster_utils.ConvertToWeiUnit(prlAmount)

			callMsg, _ := services.EthWrapper.CreateSendPRLMessage(services.MainWalletAddress,
				services.MainWalletPrivateKey,
				address, *prlAmountInWei)

			sendSuccess, _, _ := services.EthWrapper.SendPRLFromOyster(callMsg)
			if sendSuccess {
				fmt.Println("Sent successfully!")
				for {
					fmt.Println("Polling for PRL arrival")
					balance := services.EthWrapper.CheckPRLBalance(address)
					if balance.Int64() > 0 {
						fmt.Println("PRL arrived!")
						break
					}
					time.Sleep(10 * time.Second)
				}
			}

			fmt.Println("Now making a completed_upload")

			validChars := []rune("abcde123456789")
			genesisHashEndingChars := oyster_utils.RandSeq(10, validChars)

			completedUpload := models.CompletedUpload{
				GenesisHash:   qaGenHashStartingChars + genesisHashEndingChars,
				ETHAddr:       address.String(),
				ETHPrivateKey: griftPrivateKey,
			}

			vErr, err := models.DB.ValidateAndSave(&completedUpload)
			completedUpload.EncryptSessionEthKey()

			if len(vErr.Errors) > 0 {
				err := errors.New("validation errors making completed upload!")
				fmt.Println(err)
				return err
			}
			if err != nil {
				fmt.Println(err)
				return err
			}
			fmt.Println("Successfully created a completed_upload!")
		}
		return nil
	})

	grift.Desc("print_completed_uploads", "Prints the completed uploads")
	grift.Add("print_completed_uploads", func(c *grift.Context) error {

		completedUploads := []models.CompletedUpload{}

		err := models.DB.RawQuery("SELECT * from completed_uploads").All(&completedUploads)

		if err == nil {
			for _, completedUpload := range completedUploads {
				fmt.Println("Genesis hash:      " + completedUpload.GenesisHash)
				fmt.Println("ETH Address:       " + completedUpload.ETHAddr)
				fmt.Println("ETH Key:           " + completedUpload.ETHPrivateKey)
				decrypted := completedUpload.DecryptSessionEthKey()
				fmt.Println("decrypted ETH Key: " + decrypted)
				fmt.Println("PRL Status:        " + models.PRLClaimStatusMap[completedUpload.PRLStatus])
				fmt.Println("Gas Status:        " + models.GasTransferStatusMap[completedUpload.GasStatus])
				fmt.Println("________________________________________________________")
			}
		} else {
			fmt.Println(err)
		}

		return nil
	})

	grift.Desc("delete_completed_uploads", "Deletes the completed uploads")
	grift.Add("delete_completed_uploads", func(c *grift.Context) error {

		err := models.DB.RawQuery("DELETE from completed_uploads WHERE genesis_hash " +
			"LIKE " + "'" + qaGenHashStartingChars + "%';").All(&[]models.CompletedUpload{})

		if err == nil {
			fmt.Println("Completed uploads deleted")
		}

		return nil
	})
})
