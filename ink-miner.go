/*

An ink miner mines ink and disseminates blocks

Usage:
go run ink-miner.go [server ip:port] [pubKey] [privKey]

*/

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	"crypto/rand"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net"
	"net/rpc"
	"strconv"
	"strings"
)

///////////////////////////////////////////////////////////////////////////////////////////////////////////////
//											START OF INTERFACES												 //
///////////////////////////////////////////////////////////////////////////////////////////////////////////////

type MinerToMinerInterface interface {
	ConnectToNeighbour() error
	FloodToPeers(Block) error
	HeartbeatNeighbours() error
	RegisterNeighbour() error
	ReceiveBlock(Block) error
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

type MinerFromANodeInterface interface{}

type IMinerInterface interface {
	// or []byte ??

	// Just a disconnected error? other errors will be handled by methods called within mine

	Mine(chan Operation, chan Block) (chan Block, error)
}

type BlockInterface interface{}

// methods for validation, blockchain itself
type BlockChainInterface interface {
	ValidateBlock(BlockInterface) error
	ValidateOps(BlockInterface) error
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////
//								END OF INTERFACES, START OF STRUCTS											 //
///////////////////////////////////////////////////////////////////////////////////////////////////////////////

type MinerToMiner struct{}
type MinerToServer struct{}
type IMiner struct {
	serverClient *rpc.Client
	localAddr    net.Addr
	neighbours   map[net.Addr]*rpc.Client

	settings MinerNetSettings

	tails        []*Block
	currentBlock *Block
	key          ecdsa.PrivateKey
}

type Block struct {
	PrevHash string
	Ops      []Operation
	MinedBy  ecdsa.PublicKey
	Nonce    string
}

type Operation struct {
	svg   string
	owner ecdsa.PublicKey
}

/*type BlockNode struct {
	Block Block
	Children []BlockNode
	Parent BlockNode
}*/

///////////////////////////////////////////////////////////////////////////////////////////////////////////////
//								END OF STRUCTS, START OF METHODS											 //
///////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (m2m *MinerToMiner) ConnectToNeighbour() (err error) {
	return
}

func (m2m *MinerToMiner) FloodToPeers(Block) (err error) {
	return
}

func (m2m2 *MinerToMiner) ListenHB(inc string, out *string) (err error) {
	return
}

func (m2m *MinerToMiner) HeartbeatNeighbours() (err error) {
	return
}
func (m2m *MinerToMiner) RegisterNeighbour() (err error) {
	return
}
func (m2m *MinerToMiner) ReceiveBlock(Block) (err error) {
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

// Register makes RPC Register(localAddr, pubKey) call, and registers settings returned for canvas or returns error
func (m2s *MinerToServer) Register() (err error) {
	fmt.Println(ink.localAddr)
	m := &MinerInfo{
		Address: ink.localAddr,
		Key:     ink.key.PublicKey,
	}
	var settings MinerNetSettings
	err = ink.serverClient.Call("RServer.Register", m, &settings)
	if err != nil {
		return nil
	}
	ink.settings = settings
	return
}

// GetNodes makes RPC GetNodes(pubKey) call, makes a call to ConnectToNeighbour for each returned addr, can return errors
func (m2s *MinerToServer) GetNodes() (err error) {
	minerAddresses := make([]net.Addr, 0, math.MaxUint16)
	err = ink.serverClient.Call("RServer.GetNodes", ink.key.PublicKey, &minerAddresses)

	for _, addr := range minerAddresses {
		client, err := rpc.Dial("tcp", addr.String())
		if err == nil {
			ink.neighbours[addr] = client
		}
	}
	return
}

// HeartbeatServer makes RPC HearBeat(pubKey) call, changes connected state accordingly which will return different errors for art node requests
func (m2s *MinerToServer) HeartbeatServer() (err error) {
	// Create a struct, that mimics all methods provided by interface.
	// It is not compulsory, we are doing it here, just to simulate a traditional method call.
	var ignored bool
	//client.Call("RServer.HeartBeat", nil, &ignored)
	err = ink.serverClient.Call("RServer.HeartBeat", ink.key.PublicKey, &ignored)
	//	fmt.Println("Sent HB:", ignored, err)
	return
}

func (ink IMiner) Mine(newOps chan Operation, newBlock chan Block) (foundBlock chan Block, err error) {
	foundBlock = make(chan Block)
	var i uint64 = 0
	opQueue := make([]Operation, 0)
	difficulty := ink.settings.PoWDifficultyNoOpBlock

	var currentBlock Block

	go func() {
		for {
			select {
			case b := <-newBlock:
				currentBlock = Block{
					PrevHash: block2hash(&b),
					MinedBy:  ink.key.PublicKey,
					Ops:      append(currentBlock.Ops, opQueue...),
				}
				opQueue = make([]Operation, 0)
				difficulty = ink.settings.PoWDifficultyNoOpBlock
				if len(currentBlock.Ops) != 0 {
					difficulty = ink.settings.PoWDifficultyOpBlock
				}
				i = 0

			case o := <-newOps:
				opQueue = append(opQueue, o)

			default:
				i++
				currentBlock.Nonce = strconv.FormatUint(i, 10)
				if validateBlock(&currentBlock, difficulty) {
					// successfully found nonce
					log.Printf("found nonce: %s", currentBlock.Nonce)
					foundBlock <- currentBlock                                                            // spit out the found block via channel
					currentBlock = Block{PrevHash: block2hash(&currentBlock), MinedBy: ink.key.PublicKey} // TODO: change inkTransaction
					i = 0
				}
			}
		}
	}()
	return foundBlock, nil
}

var ink IMiner
var miner2server MinerToServer
var miner2miner MinerToMiner
var blocks map[string]Block
var bufferedOps []Operation

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

func killFriends() {
	for addr, miner := range ink.neighbours {
		err := miner.Call("MinerToMiner.ListenHB", "", nil)
		if err != nil {
			delete(ink.neighbours, addr)
		}
	}
}

func getBlockWithHash(hash string) *Block {
	block, ok := blocks[hash]
	if !ok {
		// Get block from neighbours
		// block = fromneighbour()
	}
	return &block
}

func getChildren(block *Block) []*Block {
	hash := block2hash(block)
	children := make([]*Block, 0, math.MaxUint16)
	for _, b := range ink.tails {
		for {
			if b.PrevHash == hash {
				children = append(children, b)
				break
			} else {
				b = getBlockWithHash(b.PrevHash)
			}
		}
	}
	return children
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
	bufferedOps = make([]Operation, 0, math.MaxUint16)
	ink = IMiner{
		serverClient: client,
		key:          *priv,
		localAddr:    l,
		neighbours:   make(map[net.Addr]*rpc.Client),
	}

	// // Register with server
	// miner2server.Register()
	// err = miner2server.GetNodes()

	// genesisBlock := Block{
	// 	PrevHash: ink.settings.GenesisBlockHash,
	// 	MinedBy:  priv.PublicKey,
	// }

	// fmt.Println(err, ink.neighbours)
	// _ = genesisBlock
	// Start mining
	// newOpsCH := make(chan Operation)
	// newBlockCH := make(chan Block)
	// foundBlockCH, err := ink.Mine(newOpsCH, newBlockCH)

	// newBlockCH <- genesisBlock

	// h := <-foundBlockCH
	// fmt.Println(h, h.PrevHash, genesisBlock)

	// Listen incoming RPC calls from artnodes
	listenForClients()

	// Heartbeat server
	// for {
	// 	go miner2server.HeartbeatServer()
	// 	go miner2miner.HeartbeatNeighbours()

	// 	time.Sleep(time.Millisecond * 50)
	// }
}

type RMiner int

func (m *RMiner) OpenCanvas(args string, reply *int) error {
	fmt.Println("New ArtNode connecting")

	return nil
}

func listenForClients() {

	artServer := rpc.NewServer()
	rminer := new(RMiner)
	artServer.Register(rminer)
	l, err := net.Listen("tcp", "127.0.0.1:9878") // get address from global ink
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Artserver started. Receiving on %s\n", ink.localAddr)
	for {
		conn, _ := l.Accept()
		go artServer.ServeConn(conn)
	}
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////
//								END OF METHODS, START OF MINING											 //
///////////////////////////////////////////////////////////////////////////////////////////////////////////////

func block2hash(block *Block) string {
	hasher := md5.New()
	hasher.Write([]byte(block2string(block)))
	return hex.EncodeToString(hasher.Sum(nil))
}

func block2string(block *Block) string {
	res1B, err := json.Marshal(*block)
	if err != nil {
		log.Println(err)
		return ""
	}
	return string(res1B)
}

func validateBlock(block *Block, difficulty uint8) bool {
	return strings.HasSuffix(block2hash(block), strings.Repeat("0", int(difficulty)))
}
