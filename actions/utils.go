package actions

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/gobuffalo/pop/nulls"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
)

const (
	// One chunk unit as represents as 1KB
	FileChunkSizeInByte = 1000

	// Number of 1KB chunk in one Sector
	FileSectorInChunkSize = 1000000

	// Separator to join []int array
	IntsJoinDelim = "_"
)

// parseReqBody take a request and parses the body to the target interface.
func parseReqBody(req *http.Request, dest interface{}) (err error) {
	body := req.Body
	defer body.Close()

	bodyBytes, err := ioutil.ReadAll(body)
	if err != nil {
		return
	}
	err = json.Unmarshal(bodyBytes, dest)

	return
}

// parseResBody take a request and parses the body to the target interface.
func parseResBody(res *http.Response, dest interface{}) (err error) {
	body := res.Body
	defer body.Close()

	bodyBytes, err := ioutil.ReadAll(body)
	if err != nil {
		return
	}
	err = json.Unmarshal(bodyBytes, dest)

	return
}

func join(A []string, delim string) string {
	var buffer bytes.Buffer
	for i := 0; i < len(A); i++ {
		buffer.WriteString(A[i])
		if i != len(A)-1 {
			buffer.WriteString(delim)
		}
	}

	return buffer.String()
}

// Transforms index with correct position for insertion after considering the buried indexes.
func TransformIndexWithBuriedIndexes(index int, treasureIdxMap []int) int {
	if len(treasureIdxMap) == 0 {
		return index
	}

	// Since index starts from 0, while FileSectorInChunkSize counts from 1, thus +1.
	// We also needs to consider to each sector to save a space for Treasure, thus -1.
	sector := (index + 1) / (FileSectorInChunkSize - 1)
	if (index - sector * FileSectorInChunkSize) < treasureIdxMap[sector] {
		return index + sector
	} else {
		return index + 1 + sector
	}
}

// Randomly generate a set of indexes in each sector
func GenerateInsertedIndexesForPearl(fileSizeInByte int) []int {
	var indexes []int
	if fileSizeInByte <= 0 {
		return indexes
	}

	fileSectorInByte := FileChunkSizeInByte * (FileSectorInChunkSize - 1)
	numOfSectors := int(math.Ceil(float64(fileSizeInByte) / float64(fileSectorInByte)))
	remainderOfChunks := math.Ceil(float64(fileSizeInByte)/FileChunkSizeInByte) + float64(numOfSectors)

	for i := 0; i < numOfSectors; i++ {
		rang := int(math.Min(FileSectorInChunkSize, remainderOfChunks))
		indexes = append(indexes, rand.Intn(rang))
		remainderOfChunks = remainderOfChunks - FileSectorInChunkSize
	}
	return indexes
}

// Merge 2 different indexes into 1 indexes. Computed Merged indexes
func MergeIndexes(a []int, b []int) ([]int, error) {
	var merged []int
	if len(a) == 0 && len(b) == 0 || len(a) != len(b) {
		return nil, errors.New("Invalid input")
	}

	for i := 0; i < len(a); i++ {
		// TODO(pzhao5): figure a better way to hash it.
		merged = append(merged, (a[i]+b[i])/2)
	}
	return merged, nil
}

// Return the IdxMap for treasure to burried
func GetTreasureIdxMap(alphaIndexes []int, betaIndexs []int) nulls.String {
	mergedIndexes, err := MergeIndexes(alphaIndexes, betaIndexs)
	var idxMap nulls.String
	if err == nil {
		idxMap = nulls.String{IntsJoin(mergedIndexes, IntsJoinDelim), true}
	} else {
		idxMap = nulls.String{"", false}
	}
	return idxMap
}

// Convert an int array to a string.
func IntsJoin(a []int, delim string) string {
	var buffer bytes.Buffer
	for i := 0; i < len(a); i++ {
		buffer.WriteString(strconv.Itoa(a[i]))
		if i != len(a)-1 {
			buffer.WriteString(delim)
		}
	}
	return buffer.String()
}

// Convert an string back to int array
func IntsSplit(a string, delim string) []int {
	var ints []int
	substrings := strings.Split(a, delim)
	for i := 0; i < len(substrings); i++ {
		v, e := strconv.Atoi(substrings[i])
		if e == nil {
			ints = append(ints, v)
		}
	}
	return ints
}
