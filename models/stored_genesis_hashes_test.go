package models_test

import (
	"github.com/oysterprotocol/brokernode/models"
	"time"
)

func (ms *ModelSuite) Test_GetGenesisHashForWebnode_no_new_genesis_hashes() {

	existingGenesisHashes := []string{
		"abcdef11",
		"abcdef12",
		"abcdef13",
	}

	storedGenesisHash1 := models.StoredGenesisHash{
		GenesisHash:    "abcdef11",
		FileSizeBytes:  5000,
		NumChunks:      5,
		WebnodeCount:   0,
		Status:         models.StoredGenesisHashUnassigned,
		TreasureStatus: models.TreasureBuried,
	}

	storedGenesisHash2 := models.StoredGenesisHash{
		GenesisHash:    "abcdef11",
		FileSizeBytes:  5000,
		NumChunks:      5,
		WebnodeCount:   0,
		Status:         models.StoredGenesisHashUnassigned,
		TreasureStatus: models.TreasureBuried,
	}

	vErr, err := ms.DB.ValidateAndCreate(&storedGenesisHash1)
	ms.Nil(err)
	ms.False(vErr.HasAny())

	vErr, err = ms.DB.ValidateAndCreate(&storedGenesisHash2)
	ms.Nil(err)
	ms.False(vErr.HasAny())

	storedGenesisHash, err := models.GetGenesisHashForWebnode(existingGenesisHashes)

	ms.Equal(models.StoredGenesisHash{}, storedGenesisHash)
	ms.Equal(models.NoGenHashesMessage, err.Error())
}

func (ms *ModelSuite) Test_GetGenesisHashForWebnode_none_unassigned() {

	existingGenesisHashes := []string{
		"abcdef11",
		"abcdef12",
		"abcdef13",
	}

	storedGenesisHash1 := models.StoredGenesisHash{
		GenesisHash:    "aaaaaa",
		FileSizeBytes:  5000,
		NumChunks:      5,
		WebnodeCount:   0,
		Status:         models.StoredGenesisHashAssigned, // assigned already
		TreasureStatus: models.TreasureBuried,
	}

	vErr, err := ms.DB.ValidateAndCreate(&storedGenesisHash1)
	ms.Nil(err)
	ms.False(vErr.HasAny())

	storedGenesisHash, err := models.GetGenesisHashForWebnode(existingGenesisHashes)

	ms.Equal(models.StoredGenesisHash{}, storedGenesisHash)
	ms.Equal(models.NoGenHashesMessage, err.Error())
}

func (ms *ModelSuite) Test_GetGenesisHashForWebnode_none_below_webnode_count_limit() {

	existingGenesisHashes := []string{
		"abcdef11",
		"abcdef12",
		"abcdef13",
	}

	storedGenesisHash1 := models.StoredGenesisHash{
		GenesisHash:    "aaaaaa",
		FileSizeBytes:  5000,
		NumChunks:      5,
		WebnodeCount:   models.WebnodeCountLimit + 1, // over the limit
		Status:         models.StoredGenesisHashUnassigned,
		TreasureStatus: models.TreasureBuried,
	}

	vErr, err := ms.DB.ValidateAndCreate(&storedGenesisHash1)
	ms.Nil(err)
	ms.False(vErr.HasAny())

	storedGenesisHash, err := models.GetGenesisHashForWebnode(existingGenesisHashes)

	ms.Equal(models.StoredGenesisHash{}, storedGenesisHash)
	ms.Equal(models.NoGenHashesMessage, err.Error())
}

func (ms *ModelSuite) Test_GetGenesisHashForWebnode_success() {

	existingGenesisHashes := []string{
		"abcdef11",
		"abcdef12",
		"abcdef13",
	}

	genesisHash := "aaaaaa"

	storedGenesisHash1 := models.StoredGenesisHash{
		GenesisHash:    genesisHash,
		FileSizeBytes:  5000,
		NumChunks:      5,
		WebnodeCount:   0,
		Status:         models.StoredGenesisHashUnassigned,
		TreasureStatus: models.TreasureBuried,
	}

	vErr, err := ms.DB.ValidateAndCreate(&storedGenesisHash1)
	ms.Nil(err)
	ms.False(vErr.HasAny())

	storedGenesisHash, err := models.GetGenesisHashForWebnode(existingGenesisHashes)

	ms.Equal(storedGenesisHash1.GenesisHash, storedGenesisHash.GenesisHash)
	ms.Nil(err)
}

func (ms *ModelSuite) Test_GetGenesisHashForWebnode_success_return_oldest() {

	existingGenesisHashes := []string{
		"abcdef11",
		"abcdef12",
		"abcdef13",
	}

	newerGenesisHash := "aaaaaa"
	olderGenesisHash := "bbbbbb"

	storedGenesisHash1 := models.StoredGenesisHash{
		GenesisHash:    newerGenesisHash,
		FileSizeBytes:  5000,
		NumChunks:      5,
		WebnodeCount:   0,
		Status:         models.StoredGenesisHashUnassigned,
		TreasureStatus: models.TreasureBuried,
	}

	storedGenesisHash2 := models.StoredGenesisHash{
		GenesisHash:    olderGenesisHash,
		FileSizeBytes:  5000,
		NumChunks:      5,
		WebnodeCount:   0,
		Status:         models.StoredGenesisHashUnassigned,
		TreasureStatus: models.TreasureBuried,
	}

	vErr, err := ms.DB.ValidateAndCreate(&storedGenesisHash1)
	ms.Nil(err)
	ms.False(vErr.HasAny())

	vErr, err = ms.DB.ValidateAndCreate(&storedGenesisHash2)
	ms.Nil(err)
	ms.False(vErr.HasAny())

	// forcibly updated the "created_at" time for the second stored_genesis_hash so it will be
	// older
	err = ms.DB.RawQuery("UPDATE stored_genesis_hashes SET created_at = ? WHERE genesis_hash = ?",
		time.Now().Add(-20*time.Second), storedGenesisHash2.GenesisHash).All(&[]models.StoredGenesisHash{})
	ms.Nil(err)

	storedGenesisHash, err := models.GetGenesisHashForWebnode(existingGenesisHashes)

	ms.Equal(olderGenesisHash, storedGenesisHash.GenesisHash)
	ms.Nil(err)
}
