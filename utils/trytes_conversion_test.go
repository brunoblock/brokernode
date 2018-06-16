package oyster_utils_test

import (
	"github.com/iotaledger/giota"
	"github.com/oysterprotocol/brokernode/utils"
	"testing"
)

type tryteConversion struct {
	b []byte
	s string
	t giota.Trytes
}

type hashAddressConversion struct {
	hash    string
	address string
}

var (
	caseOneTrytes, _   = giota.ToTrytes("IC")
	caseTwoTrytes, _   = giota.ToTrytes("HDWCXCGDEAXCGDEAPCEAHDTCGDHD")
	caseThreeTrytes, _ = giota.ToTrytes("QBCD9DPCBDVCEAXCGDEAHDWCTCEAQCTCGDHDEA9DPCBDVCFA")
	stringConvCases    = []tryteConversion{
		{b: []byte("Z"), s: "Z", t: caseOneTrytes},
		{b: []byte("this is a test"), s: "this is a test", t: caseTwoTrytes},
		{b: []byte("Golang is the best lang!"), s: "Golang is the best lang!",
			t: caseThreeTrytes},
		{b: []byte(""), s: "", t: ""},
	}
)

var hashAddressConvCases = []hashAddressConversion{
	{hash: "5804c3157e3de4e4a8b1f2417d8c61454e368883ec05e32f234386690e7c9696",
		address: "GCD9FGU9RDGBLHLHFFOFZHKBQDEEPCOBXB9BAEWDTHE9KHTAHAMBZDXCN9PDOEOE99999999999999999"},
	{hash: "080779a63f5822c2606bfdd2801b5c4429918efcecffbaa34c2daadd51bc5748",
		address: "H9G9MDDFIBGCGAEGOCZCJIUGTD9AKCNBNAJEGEIITHLIXFAFVBRAHFEH9CZFFCRB99999999999999999"},
	{hash: "d0199d3bd44c9301299de4d9d7054adb9c7fa11ac175cdee302794130b081681",
		address: "SGY9VEEBWGVBLEA9NAVELHAHZGE9TBCHUESDZEZ9DGIDPGVHUALAMES9K9H9V9UD99999999999999999"},
	{hash: "e512f80fa0e0c2872e0e29e621c40cf1693e112e020a708a619e7b87d421bf9c",
		address: "MHR9EIO9YEHHEG9ESAN9NANHFAGGL9YHXCHBQ9SAB9J9DDCEPCWEOD9EWGFABGUE99999999999999999"},
	{hash: "cca31d69bcddfdd0ecd53d98c3daeca17ed61e04bf456ebd56b9ddbaf660091a",
		address: "OGAFBAXCZFEHJISGTHXGGBQEFGBHTHZERDYGCAD9BGOBBD9GECWFEHXFCIOCI9Z999999999999999999"},
}

func Test_BytesToTrytes(t *testing.T) {
	for _, tc := range stringConvCases {
		result := oyster_utils.BytesToTrytes([]byte(tc.b))
		if result != tc.t {
			t.Fatalf("BytesToTrytes(%q) should be %#v but returned %s",
				tc.b, tc.t, result)
		}
	}
}

func Test_TrytesToBytes(t *testing.T) {
	for _, tc := range stringConvCases {
		if string(oyster_utils.TrytesToBytes(tc.t)) != string(tc.b) {
			t.Fatalf("TrytesToBytes(%q) should be %#v but returned %s",
				tc.t, tc.b, oyster_utils.TrytesToBytes(tc.t))
		}
	}
}

func Test_TrytesToAsciiTrimmed(t *testing.T) {
	for _, tc := range stringConvCases {
		result, _ := oyster_utils.TrytesToAsciiTrimmed(string(tc.t))
		if result != string(tc.s) {
			t.Fatalf("TrytesToAsciiTrimmed(%q) should be %#v but returned %s",
				tc.t, tc.s, result)
		}
	}
}

func Test_AsciiToTrytes(t *testing.T) {
	for _, tc := range stringConvCases {
		result, _ := oyster_utils.AsciiToTrytes(tc.s)
		if result != string(tc.t) {
			t.Fatalf("AsciiToTrytes(%q) should be %#v but returned %s",
				tc.s, tc.t, result)
		}
	}
}

func Test_MakeAddress(t *testing.T) {
	for _, tc := range hashAddressConvCases {
		result := oyster_utils.MakeAddress(tc.hash)
		if result != string(tc.address) {
			t.Fatalf("MakeAddress(%q) should be %#v but returned %s",
				tc.hash, tc.address, result)
		}
	}
}
