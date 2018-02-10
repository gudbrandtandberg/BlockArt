/*

An ink miner mines ink and disseminates blocks

Usage:
go run ink-miner.go [server ip:port] [pubKey] [privKey]

*/

package main

import (
	"fmt"
	"net/rpc"
	"flag"
	"time"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/gob"
	"net"
	"crypto/rand"
	"bytes"
	"reflect"
	"encoding/json"
	"crypto/md5"
	"encoding/hex"
	"strings"
	"math"
	"strconv"
	"log"
)

///////////////////////////////////////////////////////////////////////////////////////////////////////////////
//											START OF INTERFACES												 //
///////////////////////////////////////////////////////////////////////////////////////////////////////////////

type MinerToMinerInterface interface {
	ConnectToNeighbour() error
	FloodToPeers() error
	HeartbeatNeighbours() error
	RegisterNeighbour() error
	ReceiveAndFlood() error
}

////// TCP RPC calls to make against server
/*
settings, err ← Register(address, publicKey)
	Registers a new miner witMinerToMinerh an address for other miner to use to connect to it (returned in GetNodes call below)
		and a public-key for this miner. Returns error, or if error is not set, then setting for this canvas instance.
	Returns AddressAlreadyRegisteredError if the server has already registered this address. 
	Returns KeyAlreadyRegisteredError if the server already has a registration record for publicKey.
addrSet,err ← GetNodes(publicKey)
	Returns addresses for a subset of miners in the system. Returns UnknownKeyError if the server does not know 
		a miner with this publicKey.
err ← HeartBeat(publicKey)
	The server also listens for heartbeats from known miners. A miner must send a heartbeat to the server 
		every HeartBeat milliseconds (specified in settings from server) after calling Register, otherwise the server 
		will stop returning this miner's address/key to other miners. 
	Returns UnknownKeyError if the server does not know a miner with this publicKey.
*/
type MinerToServerInterface interface {
	// makes RPC Register(localAddr, pubKey) call, and registers settings returned for canvas or returns error
	Register() error

	// Makes RPC GetNodes(pubKey) call, makes a call to ConnectToNeighbour for each returned addr, can return errors
	GetNodes() error

	// makes RPC HearBeat(pubKey) call, changes connected state accordingly which will return different errors for art node requests
	HeartbeatServer() error
}

type MinerFromANodeInterface interface {}

type IMinerInterface interface {
	// or []byte ??

	// Just a disconnected error? other errors will be handled by methods called within mine

	Mine() error
}

type BlockInterface interface {}

// methods for validation, blockchain itself
type BlockChainInterface interface {
	ValidateBlock(BlockInterface) error
	ValidateOps(BlockInterface) error
}


///////////////////////////////////////////////////////////////////////////////////////////////////////////////
//								END OF INTERFACES, START OF STRUCTS											 //
///////////////////////////////////////////////////////////////////////////////////////////////////////////////

type MinerToMiner struct {}
type MinerToServer struct {}
type IMiner struct {
	serverClient *rpc.Client
	localAddr net.Addr
	neighbours map[net.Addr]*rpc.Client

	key ecdsa.PrivateKey
}

type Operation struct {
	svg string
	owner ecdsa.PublicKey
}

type Block struct {
	prevHash string
	minedBy ecdsa.PublicKey
	ops []Operation
	nonce string
}


///////////////////////////////////////////////////////////////////////////////////////////////////////////////
//								END OF STRUCTS, START OF METHODS											 //
///////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (m2m *MinerToMiner) ConnectToNeighbour() (err error) {
	return
}

func (m2m *MinerToMiner) FloodToPeers() (err error) {
	return
}

func (m2m2 *MinerToMiner) ListenHB(inc string, out *string) (err error) {}

func (m2m *MinerToMiner) HeartbeatNeighbours() (err error) {
	return
}
func (m2m *MinerToMiner) RegisterNeighbour() (err error) {
	return
}
func (m2m *MinerToMiner) ReceiveAndFlood() (err error) {
	return
}

type MinerInfo struct {
	Address net.Addr
	Key     ecdsa.PublicKey
}

type MinerSettings struct {
	// Hash of the very first (empty) block in the chain.
	GenesisBlockHash string `json:"genesis-block-hash"`

	// The minimum number of ink miners that an ink miner should be
	// connected to.
	MinNumMinerConnections uint8 `json:"min-num-miner-connections"`

	// Mining ink reward per op and no-op blocks (>= 1)
	InkPerOpBlock   uint32 `json:"ink-per-op-block"`
	InkPerNoOpBlock uint32 `json:"ink-per-no-op-block"`

	// Number of milliseconds between heartbeat messages to the server.
	HeartBeat uint32 `json:"heartbeat"`

	// Proof of work difficulty: number of zeroes in prefix (>=0)
	PoWDifficultyOpBlock   uint8 `json:"pow-difficulty-op-block"`
	PoWDifficultyNoOpBlock uint8 `json:"pow-difficulty-no-op-block"`
}

type MinerNetSettings struct {
	MinerSettings

	// Canvas settings
	CanvasSettings CanvasSettings `json:"canvas-settings"`
}

type CanvasSettings struct {
	// Canvas dimensions
	CanvasXMax uint32 `json:"canvas-x-max"`
	CanvasYMax uint32 `json:"canvas-y-max"`
}

// makes RPC Register(localAddr, pubKey) call, and registers settings returned for canvas or returns error
func (m2s *MinerToServer) Register() (err error) {
	fmt.Println(ink.localAddr)
	m := &MinerInfo{
		Address: ink.localAddr,
		Key: ink.key.PublicKey,
	}
	var settings MinerNetSettings
	err = ink.serverClient.Call("RServer.Register", m, &settings)
	fmt.Println("Register", settings, err)
	return
}

// Makes RPC GetNodes(pubKey) call, makes a call to ConnectToNeighbour for each returned addr, can return errors
func (m2s *MinerToServer) GetNodes() (err error) {
	minerAddresses := make([]net.Addr, 0, 65535)
	err = ink.serverClient.Call("RServer.GetNodes", ink.key.PublicKey, &minerAddresses)

	for _, addr := range minerAddresses {
		client, err := rpc.Dial("tcp", addr.String())
		if err == nil {
			ink.neighbours[addr] = client
		}
	}
	return
}

// makes RPC HearBeat(pubKey) call, changes connected state accordingly which will return different errors for art node requests
func (m2s *MinerToServer) HeartbeatServer() (err error) {
	// Create a struct, that mimics all methods provided by interface.
	// It is not compulsory, we are doing it here, just to simulate a traditional method call.
	var ignored bool
	//client.Call("RServer.HeartBeat", nil, &ignored)
	err = ink.serverClient.Call("RServer.HeartBeat", ink.key.PublicKey, &ignored)
	fmt.Println("Sent HB:", ignored, err)
	return
}

func (ink IMiner) Mine() (err error) {
	return
}

var ink IMiner
var miner2server MinerToServer
var miner2miner MinerToMiner

func tmp() net.Addr {
	server := rpc.NewServer()

	l, _ := net.Listen("tcp", ":0")

	go func() {
		for {
			conn, _ := l.Accept()
			go server.ServeConn(conn)
		}
	}()

	return l.Addr()
}

func killFriends () {
	for addr, miner := range ink.neighbours {
		err := miner.Call("MinerToMiner.ListenHB", "", nil)
		if err != nil {
			delete(ink.neighbours, addr)
		}
	}
}

func main() {
	gob.Register(&net.TCPAddr{})
	gob.Register(&elliptic.CurveParams{})
	gob.Register(&MinerInfo{})

	priv, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		fmt.Println(err)
	}

	ipPort := flag.String("i", "127.0.0.1:12345", "RPC server ip:port")
	client, err := rpc.Dial("tcp", *ipPort)

	l := tmp()

	ink = IMiner{
		serverClient: client,
		key: *priv,
		localAddr: l,
		neighbours: make(map[net.Addr]*rpc.Client),
	}
	
	// Register with server
	miner2server.Register()
	err = miner2server.GetNodes()

	fmt.Println(err, ink.neighbours)

	// Heartbeat server
	for {
		go miner2server.HeartbeatServer()
		go miner2miner.HeartbeatNeighbours()

		time.Sleep(time.Millisecond * 50)
	}
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////
//								END OF METHODS, START OF MINING											 //
///////////////////////////////////////////////////////////////////////////////////////////////////////////////

func block2string(block Block) string {
	res1B, err := json.Marshal(block)
	if err != nil {
		log.Println(err)
		return ""
	}
	return string(res1B)
}

func mineBlock(block Block, difficulty int) string {
	var i uint64
	stringBlock := block2string(block)

	for i = 0; i < math.MaxUint64; i++ {
		nonce := strconv.FormatUint(i, 10)

		if validateNonce(stringBlock, nonce, difficulty) {
			return nonce
		}
	}
	return ""
}

func validateBlock(block Block, nonce string, difficulty int) bool {
	return validateNonce(block2string(block), nonce, difficulty)
}

func validateNonce(stringBlock string, nonce string, difficulty int) bool {
	hasher := md5.New()
	hasher.Write([]byte(stringBlock + nonce))
	hash := hex.EncodeToString(hasher.Sum(nil))

	return strings.HasSuffix(hash, strings.Repeat("0", difficulty))
}
