/*

Implements an example server for the BlockArt project, to be used in
project 1 of UBC CS 416 2017W2.

This server takes in settings from an input json files and implements
a simple strategy for GetNodes: return a fixed number of random miners
("num-miner-to-return" in the json config file).

Usage:

$ go run server.go
  -c string
    	Path to the JSON config

*/

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/gob"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"sort"
	"sync"
	"time"
)

// Errors that the server could return.
type UnknownKeyError error

type KeyAlreadyRegisteredError string

func (e KeyAlreadyRegisteredError) Error() string {
	return fmt.Sprintf("BlockArt server: key already registered [%s]", string(e))
}

type AddressAlreadyRegisteredError string

func (e AddressAlreadyRegisteredError) Error() string {
	return fmt.Sprintf("BlockArt server: address already registered [%s]", string(e))
}

// Settings for a canvas in BlockArt.
type CanvasSettings struct {
	// Canvas dimensions
	CanvasXMax uint32 `json:"canvas-x-max"`
	CanvasYMax uint32 `json:"canvas-y-max"`
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

// Settings for an instance of the BlockArt project/network.
type MinerNetSettings struct {
	MinerSettings

	// Canvas settings
	CanvasSettings CanvasSettings `json:"canvas-settings"`
}

type RServer int

type Miner struct {
	Address         net.Addr
	RecentHeartbeat int64
}

type Config struct {
	MinerSettings    MinerNetSettings `json:"miner-settings"`
	RpcIpPort        string           `json:"rpc-ip-port"`
	NumMinerToReturn uint8            `json:"num-miner-to-return"`
}

type AllMiners struct {
	sync.RWMutex
	all map[string]*Miner
}

var (
	unknownKeyError UnknownKeyError = errors.New("BlockArt server: unknown key")
	config          Config
	errLog          *log.Logger = log.New(os.Stderr, "[serv] ", log.Lshortfile|log.LUTC|log.Lmicroseconds)
	outLog          *log.Logger = log.New(os.Stderr, "[serv] ", log.Lshortfile|log.LUTC|log.Lmicroseconds)
	// Miners in the system.
	allMiners AllMiners = AllMiners{all: make(map[string]*Miner)}
)

func readConfigOrDie(path string) {
	file, err := os.Open(path)
	handleErrorFatal("config file", err)

	buffer, err := ioutil.ReadAll(file)
	handleErrorFatal("read config", err)

	err = json.Unmarshal(buffer, &config)
	handleErrorFatal("parse config", err)
}

// These variables control whether we should be working with the webserver
const (
	BROADCAST = true
	GENERATE  = true
)

// Parses args, setups up RPC server.
func main() {
	gob.Register(&net.TCPAddr{})
	gob.Register(&elliptic.CurveParams{})

	path := flag.String("c", "config.json", "Path to the JSON config")
	flag.Parse()

	if *path == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	readConfigOrDie(*path)

	// Start listening for / broadcasting new blocks
	newBlockCh := make(chan Path) // should contain blocks in the future
	go generateNewBlocks(newBlockCh)
	go broadcastNewBlocks(newBlockCh)

	rand.Seed(time.Now().UnixNano())

	rserver := new(RServer)

	server := rpc.NewServer()
	server.Register(rserver)

	l, e := net.Listen("tcp", config.RpcIpPort)

	handleErrorFatal("listen error", e)
	outLog.Printf("Server started. Receiving on %s\n", config.RpcIpPort)

	for {
		conn, _ := l.Accept()
		go server.ServeConn(conn)
	}
}

type MinerInfo struct {
	Address net.Addr
	Key     ecdsa.PublicKey
}

// Function to delete dead miners (no recent heartbeat)
func monitor(k string, heartBeatInterval time.Duration) {
	for {
		allMiners.Lock()
		if time.Now().UnixNano()-allMiners.all[k].RecentHeartbeat > int64(heartBeatInterval) {
			fmt.Println("timed out",
				int64(time.Now().UnixNano()),
				int64(allMiners.all[k].RecentHeartbeat),
				int64(heartBeatInterval),
				allMiners.all[k].Address.String())
			delete(allMiners.all, k)
			allMiners.Unlock()
			return
		}
		outLog.Printf("%s is alive\n", allMiners.all[k].Address.String())
		allMiners.Unlock()
		time.Sleep(heartBeatInterval)
	}
}

func pubKeyToString(key ecdsa.PublicKey) string {
	return string(elliptic.Marshal(key.Curve, key.X, key.Y))
}

// Registers a new miner with an address for other miner to use to
// connect to it (returned in GetNodes call below), and a
// public-key for this miner. Returns error, or if error is not set,
// then setting for this canvas instance.
//
// Returns:
// - AddressAlreadyRegisteredError if the server has already registered this address.
// - KeyAlreadyRegisteredError if the server already has a registration record for publicKey.
func (s *RServer) Register(m MinerInfo, r *MinerNetSettings) error {
	allMiners.Lock()
	defer allMiners.Unlock()

	k := pubKeyToString(m.Key)
	if miner, exists := allMiners.all[k]; exists {
		return KeyAlreadyRegisteredError(miner.Address.String())
	}

	for _, miner := range allMiners.all {
		if miner.Address.Network() == m.Address.Network() && miner.Address.String() == m.Address.String() {
			return AddressAlreadyRegisteredError(m.Address.String())
		}
	}

	allMiners.all[k] = &Miner{
		m.Address,
		time.Now().UnixNano(),
	}

	go monitor(k, time.Duration(config.MinerSettings.HeartBeat)*time.Millisecond)

	*r = config.MinerSettings

	outLog.Printf("Got Register from %s\n", m.Address.String())

	return nil
}

type Addresses []net.Addr

func (a Addresses) Len() int           { return len(a) }
func (a Addresses) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a Addresses) Less(i, j int) bool { return a[i].String() < a[j].String() }

// Returns addresses for a subset of miners in the system.
//
// Returns:
// - UnknownKeyError if the server does not know a miner with this publicKey.
func (s *RServer) GetNodes(key ecdsa.PublicKey, addrSet *[]net.Addr) error {

	// TODO: validate miner's GetNodes protocol? (could monitor state
	// of network graph/connectivity and validate protocol FSM)

	allMiners.RLock()
	defer allMiners.RUnlock()

	k := pubKeyToString(key)

	if _, ok := allMiners.all[k]; !ok {
		return unknownKeyError
	}

	minerAddresses := make([]net.Addr, 0, len(allMiners.all)-1)

	for pubKey, miner := range allMiners.all {
		if pubKey == k {
			continue
		}
		minerAddresses = append(minerAddresses, miner.Address)
	}

	sort.Sort(Addresses(minerAddresses))

	deterministicRandomNumber := key.X.Int64() % 32
	r := rand.New(rand.NewSource(deterministicRandomNumber))
	for n := len(minerAddresses); n > 0; n-- {
		randIndex := r.Intn(n)
		minerAddresses[n-1], minerAddresses[randIndex] = minerAddresses[randIndex], minerAddresses[n-1]
	}

	numMiners := math.Min(float64(len(allMiners.all)-1), float64(config.NumMinerToReturn))
	*addrSet = minerAddresses[:int64(numMiners)]

	return nil
}

// The server also listens for heartbeats from known miners. A miner must
// send a heartbeat to the server every HeartBeat milliseconds
// (specified in settings from server) after calling Register, otherwise
// the server will stop returning this miner's address/key to other
// miners.
//
// Returns:
// - UnknownKeyError if the server does not know a miner with this publicKey.
func (s *RServer) HeartBeat(key ecdsa.PublicKey, _ignored *bool) error {
	fmt.Println("Received HB:")

	allMiners.Lock()
	defer allMiners.Unlock()

	k := pubKeyToString(key)
	if _, ok := allMiners.all[k]; !ok {
		return unknownKeyError
	}

	allMiners.all[k].RecentHeartbeat = time.Now().UnixNano()

	return nil
}

func handleErrorFatal(msg string, e error) {
	if e != nil {
		errLog.Fatalf("%s, err = %s\n", msg, e.Error())
	}
}

//// Gudbrand's code

const (
	webServerAddr = "127.0.0.1:7255"
	delayMean     = 4.0
	delayStd      = 3.0
)

// A Path contains fields for drawing SVG path elements
type Path struct {
	SVGString string
	Fill      string
	Stroke    string
}

// Every time a new block is made, this process will broadcast the block to the web-server
func broadcastNewBlocks(ch chan Path) {
	if !BROADCAST {
		return
	}
	// TODO: handle errors
	for {
		p := <-ch
		fmt.Println("About to tell the webserver about the new block")
		conn, err := net.Dial("tcp", webServerAddr)
		if err != nil {
			fmt.Println(err)
		}
		msg, err := json.Marshal(p)
		if err != nil {
			fmt.Println(err)
		}
		_, err = conn.Write(msg)
		if err != nil {
			fmt.Println(err)
		}
		conn.Close()
	}
}

var blackSquare = Path{"M 10 10 h 100 v 100 h -100 v -100", "black", "black"}
var yellowSquare = Path{"M 110 110 h 100 v 100 h -100 v -100", "yellow", "black"}
var redSquare = Path{"M 210 10 h 100 v 100 h -100 v -100", "red", "black"}
var blueSquare = Path{"M 10 210 h 100 v 100 h -100 v -100", "blue", "black"}
var greenSquare = Path{"M 210 210 h 100 v 100 h -100 v -100", "green", "black"}
var commandQueue = []Path{blackSquare, yellowSquare, redSquare, blueSquare, greenSquare}

func generateNewBlocks(ch chan Path) {
	if !GENERATE {
		return
	}
	i := 0
	for {
		delay := time.Duration(math.Max(rand.NormFloat64()*delayStd+delayMean, 0.0))
		time.Sleep(delay * time.Second)
		if i < 5 {
			fmt.Printf("Issuing command #%d\n", i)
			ch <- commandQueue[i]
		}
		i++
	}
}
