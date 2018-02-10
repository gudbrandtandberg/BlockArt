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
	"time"
)

///////////////////////////////////////////////////////////////////////////////////////////////////////////////
//											START OF INTERFACES												 //
///////////////////////////////////////////////////////////////////////////////////////////////////////////////

type MinerToMinerInterface interface {
	ConnectToNeighbour() error
	FloodToPeers() error
	HeartbeatNeighbours() error
	GetHeartbeats() error
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

type MinerFromANodeInterface interface{}

type IMinerInterface interface {
	// or []byte ??

	// Just a disconnected error? other errors will be handled by methods called within mine

	Mine() error
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

	key ecdsa.PrivateKey
}

type Operation struct {
	svg   string
	owner ecdsa.PublicKey
}

type Block struct {
	PrevHash string
	MinedBy  ecdsa.PublicKey
	Ops      []Operation
	Nonce    string
}

/*
type BlockNode struct {
	Block Block
	Children []BlockNode
	Parent BlockNode
}
*/

///////////////////////////////////////////////////////////////////////////////////////////////////////////////
//								END OF STRUCTS, START OF METHODS											 //
///////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (m2m *MinerToMiner) ConnectToNeighbour() (err error) {
	return
}

func (m2m *MinerToMiner) FloodToPeers() (err error) {
	return
}

func (m2m2 *MinerToMiner) GetHeartbeats(inc string, out *string) (err error) {
	*out = "hello i'm online"
	return
}

func (m2m *MinerToMiner) HeartbeatNeighbours() (err error) {
	for {
		for neighbourAddr, neighbourRPC := range ink.neighbours {
			var rep string
			timeout := make(chan error, 1)
			go func() {
				timeout <- neighbourRPC.Call("MinerToMiner.GetHeartbeats", "", &rep)
			}()
			select {
			case err := <-timeout:
				if err != nil {
					delete(ink.neighbours, neighbourAddr)
				}
			case <-time.After(1 * time.Second):
				delete(ink.neighbours, neighbourAddr)
			}
		}
		//give neighbours time to respond
		time.Sleep(2 * time.Second)
		//if we have good neighbours, return
		if len(ink.neighbours) >= int(ink.settings.MinNumMinerConnections) {
			return
		}
		//else we get more neighbours
		err = miner2server.GetNodes()
		checkError(err)
		go m2m.HeartbeatNeighbours()
		checkError(err)

	}

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

// Makes RPC GetNodes(pubKey) call, makes a call to ConnectToNeighbour for each returned addr, can return errors
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

// makes RPC HearBeat(pubKey) call, changes connected state accordingly which will return different errors for art node requests
func (m2s *MinerToServer) HeartbeatServer() (err error) {
	// Create a struct, that mimics all methods provided by interface.
	// It is not compulsory, we are doing it here, just to simulate a traditional method call.
	var ignored bool
	//client.Call("RServer.HeartBeat", nil, &ignored)
	err = ink.serverClient.Call("RServer.HeartBeat", ink.key.PublicKey, &ignored)
	//	fmt.Println("Sent HB:", ignored, err)
	return
}

func (ink IMiner) Mine() (err error) {
	var i uint64
	ink.currentBlock.Ops = bufferedOps // TODO: will this break everything?

	for i = 0; i < math.MaxUint64; i++ {
		nonce := strconv.FormatUint(i, 10)
		difficulty := ink.settings.PoWDifficultyNoOpBlock
		if len(ink.currentBlock.Ops) != 0 {
			difficulty = ink.settings.PoWDifficultyOpBlock
		}
		if validateNonce(ink.currentBlock, nonce, difficulty) {
			// Broadcast nonce
			log.Println(nonce)
			ink.Mine() // Burn the CPU
		}
	}
	log.Println("This should not happen")
	return nil
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

	// Register with server
	miner2server.Register()
	err = miner2server.GetNodes()
	checkError(err)

	genesisBlock := &Block{
		PrevHash: ink.settings.GenesisBlockHash,
		MinedBy:  priv.PublicKey,
	}
	ink.currentBlock = genesisBlock

	fmt.Println(err, ink.neighbours)

	// Heartbeat server
	for {
		go miner2server.HeartbeatServer()

		//go miner2miner.HeartbeatNeighbours()

		time.Sleep(time.Millisecond * 50)
	}
	miner2miner.HeartbeatNeighbours()
	checkError(err)
	go ink.Mine()
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

/*
Block validations:
	Check that the nonce for the block is valid: PoW is correct and has the right difficulty.
	Check that each operation in the block has a valid signature (this signature should be generated using the private key and the operation).
	Check that the previous block hash points to a legal, previously generated, block.
Operation validations:
	Check that each operation has sufficient ink associated with the public key that generated the operation.
	Check that each operation does not violate the shape intersection policy described above.
	Check that the operation with an identical signature has not been previously added to the blockchain (prevents operation replay attacks).
	Check that an operation that deletes a shape refers to a shape that exists and which has not been previously deleted.
*/

func validateBlock(block *Block, nonce string, difficulty uint8) bool {
	
	var validNonce bool
	validNonce = validateNonce(block, nonce, difficulty)
	var validOpSigs bool
	validOpSigs = true
	for _, op := range block.Ops {
		
	}
	
	return true
}

func validateNonce(block *Block, nonce string, difficulty uint8) bool {
	return strings.HasSuffix(block2hash(block), strings.Repeat("0", int(difficulty)))
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////
//								END OF MINING, START OF UTILITIES											 //
///////////////////////////////////////////////////////////////////////////////////////////////////////////////

func checkError(err error) {
	if err != nil {
		fmt.Println("ERROR:", err)
	}
}
