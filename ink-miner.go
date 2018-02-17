/*

    An ink miner mines ink and disseminates blocks

	Usage:
	go run ink-miner.go [server ip:port] [pubKey] [privKey]

*/

package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	"crypto/rand"
	"crypto/x509"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"net/rpc"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"./blockartlib"
	"math"
)

import _ "net/http/pprof"

const debugLocks = false
///////////////////////////////////////////////////////////////////////////////////////////////////////////////
//											START OF INTERFACES												 //
///////////////////////////////////////////////////////////////////////////////////////////////////////////////

type MinerToMinerInterface interface {
	FloodBlockToPeers(block *Block) error
	FloodOpToPeers(op Operation) error
	HeartbeatNeighbours() error
	GetHeartbeats() error
	GetBlockChain() error
	FetchBlockChain(i string, blockchain *[]Block) error
	ReceiveBlock(block *Block, reply *bool) error
	ReceiveOp(op Operation, reply *bool) error
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

type MinerFromANodeInterface interface {
	GetGenesisBlock(input string, hash *string) (err error)
	GetChildren(hash string, childrenHashes *[]string) (err error)
}

type IMinerInterface interface {
	// Just a disconnected error? other errors will be handled by methods called within mine
	getLongestChain() (hash string)

	Length(hash string) (err error)
	ValidationCount(hash string) (err error)
	Mine() error
}

type BlockInterface interface{}

// methods for validation, blockchain itself
type BlockChainInterface interface {
	ValidateBlock(BlockInterface) error
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////
//								END OF INTERFACES, START OF STRUCTS											 //
///////////////////////////////////////////////////////////////////////////////////////////////////////////////

type MinerToMiner struct{}
type MinerToServer struct{}
type MinerFromANode struct{}
type IMiner struct {
	serverClient *rpc.Client
	localAddr    net.Addr
	artAddr      string
	neighbours   map[string]*rpc.Client

	settings MinerNetSettings
	key      ecdsa.PrivateKey
}

type Operation struct {
	Delete  bool
	SVG     string
	SVGHash SVGHash
	Owner   ecdsa.PublicKey
	ValNum  uint8
}
type SVGHash struct {
	Hash []byte
	R, S *big.Int
}

type Block struct {
	PrevHash string
	MinedBy  ecdsa.PublicKey
	Ops      []Operation
	Nonce    string
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////
//								END OF STRUCTS, START OF METHODS											 //
///////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (art MinerFromANode) GetGenesisBlock(input string, hash *string) (err error) {
	*hash = ink.settings.GenesisBlockHash // TODO: This hash does not have a block
	return
}

type InvalidBlockHashError string

func (e InvalidBlockHashError) Error() string {
	return fmt.Sprintf("BlockArt: Invalid block hash [%s]", string(e))
}

func (art MinerFromANode) GetChildren(hash string, childrenHashes *[]string) (err error) {
	for _, block := range ink.GetChildren(hash) {
		*childrenHashes = append(*childrenHashes, block2hash(&block))
	}
	return // TODO: ERROR
}

func (m2m *MinerToMiner) FloodBlockToPeers(block *Block) (err error) {
	if len(block.Ops) > 0 {
		fmt.Println("Sent", block.Nonce, block2hash(block))
	}
//	fmt.Println(block.PrevHash, block.Nonce, block.Ops, block.MinedBy)
	m2m.HeartbeatNeighbours()

	if debugLocks { log.Println("neighbourlock1 locking") }
	neighbourlock.RLock()
	if debugLocks { log.Println("neighbourlock1 locked") }
	for _, neighbour := range ink.neighbours {
		var reply bool
		go checkError(neighbour.Call("MinerToMiner.ReceiveBlock", &block, &reply))
	}
	if debugLocks { log.Println("neighbourlock1 unlocking") }
	neighbourlock.RUnlock()
	if debugLocks { log.Println("neighbourlock1 unlocked") }
	return
}

func (m2m *MinerToMiner) FloodOpToPeers(op Operation) (err error) {
//	fmt.Println("FLOODOP", op.SVG, op.SVGHash.Hash, len(ink.neighbours))
//	fmt.Println(block.PrevHash, block.Nonce, block.Ops, block.MinedBy)
	m2m.HeartbeatNeighbours()

	if debugLocks { log.Println("neighbourlock1 locking") }
	neighbourlock.RLock()
	if debugLocks { log.Println("neighbourlock1 locked") }
	for _, neighbour := range ink.neighbours {
		var reply bool
		go checkError(neighbour.Call("MinerToMiner.ReceiveOp", &op, &reply))
	}
	if debugLocks { log.Println("neighbourlock1 unlocking") }
	neighbourlock.RUnlock()
	if debugLocks { log.Println("neighbourlock1 unlocked") }
	return
}

func (m2m *MinerToMiner) GetHeartbeats(incAddr string, out *string) (err error) {
	if debugLocks { log.Println("neighbourlock2 locking") }
	neighbourlock.RLock()
	if debugLocks { log.Println("neighbourlock2 locked") }
	_, ok := ink.neighbours[incAddr]
	if debugLocks { log.Println("neighbourlock2 unlocking") }
	neighbourlock.RUnlock()
	if debugLocks { log.Println("neighbourlock2 unlocked") }

	if !ok {
		//if neighbour doesn't exist
		go func () {
			client, err := rpc.Dial("tcp", incAddr)
			if !checkError(err) {
				fmt.Println("added:", incAddr)
				if debugLocks { log.Println("neighbourlock3 locking") }
				neighbourlock.Lock()
				if debugLocks { log.Println("neighbourlock3 locked") }
				ink.neighbours[incAddr] = client
				if debugLocks { log.Println("neighbourlock3 unlocking") }
				neighbourlock.Unlock()
				if debugLocks { log.Println("neighbourlock3 unlocked") }
			}
		}()
	}

	*out = "hello i'm online"
	return
}

func (m2m *MinerToMiner) FetchBlockChain(i string, blockchain *[]Block) (err error) {
    if debugLocks { fmt.Println("locking7") }
	maplock.RLock()
    if debugLocks { fmt.Println("locked7") }
	v := make([]Block, 0, len(blocks))
	for _, value := range blocks {
		v = append(v, value)
	}
	*blockchain = v

	if debugLocks { fmt.Println("unlocking7") }
	maplock.RUnlock()
	if debugLocks { fmt.Println("unlocked7") }
	return
}

func deleteUnresponsiveNeighbour(neighbourAddr string, neighbourRPC *rpc.Client) (err error) {
	var rep string
	err = neighbourRPC.Call("MinerToMiner.GetHeartbeats", ink.localAddr.String(), &rep)
	if checkError(err) {
		fmt.Println("delete:", neighbourAddr)
		neighbourlock.Lock()
		delete(ink.neighbours, neighbourAddr)
		neighbourlock.Unlock()
	}
	return
}

func (m2m *MinerToMiner) HeartbeatNeighbours() (err error) {
	if debugLocks { log.Println("neighbourlock5 locking") }
	neighbourlock.RLock()
	if debugLocks { log.Println("neighbourlock5 locked") }
	for neighbourAddr, neighbourRPC := range ink.neighbours {
		go deleteUnresponsiveNeighbour(neighbourAddr, neighbourRPC)
	}
	if debugLocks { log.Println("neighbourlock5 unlocking") }
	neighbourlock.RUnlock()
	if debugLocks { log.Println("neighbourlock5 unlocked") }
	//give neighbours time to respond
	time.Sleep(2 * time.Second)
	//if we have good neighbours, return
	neighbourlock.RLock()
//	fmt.Println("len neighbours, minminers, neighbours: ", len(ink.neighbours), ink.settings.MinNumMinerConnections, ink.neighbours)
	if (len(ink.neighbours) >= int(ink.settings.MinNumMinerConnections)) || (len(ink.neighbours) == 0) {
		neighbourlock.RUnlock()
		return
	}
	neighbourlock.RUnlock()
	//else we get more neighbours
	err = miner2server.GetNodes()
	return
}

func (m2m MinerToMiner) checkValidationOps() {
	if debugLocks { fmt.Println("locking13") }
	maplock.RLock()
	if debugLocks { fmt.Println("locked13") }
	operationsToReAdd := make([]Operation, 0)
	blockCopies := make(map[string]Block, 0)
	for k, v := range blocks {
		blockCopies[k] = v
	}
	if debugLocks { fmt.Println("unlocking13") }
	maplock.RUnlock()
	if debugLocks { fmt.Println("unlocked13") }

	validationMap := make(map[string]int)
	lengthMap := make(map[string]int)
	longestChainHash := ink.getLongestChain()
	tipOfChain := blockCopies[longestChainHash]
	longestChainLength := ink.Length(longestChainHash)
	length := longestChainLength
	valCount := 0
	for tipOfChain.PrevHash != ink.settings.GenesisBlockHash {
		for _, op := range tipOfChain.Ops {
			if ecdsa.Verify(&ink.key.PublicKey, op.SVGHash.Hash, op.SVGHash.R, op.SVGHash.S) {
				validationMap[string(op.SVGHash.Hash)] = valCount
				lengthMap[string(op.SVGHash.Hash)] = length
			}
		}
		length -= 1
		valCount += 1
		tipOfChain = blockCopies[tipOfChain.PrevHash]
	}
	for {
		if len(toValidateOpsCH) == 0 {
			break
		}
		opToCheck := <-toValidateOpsCH

		//fmt.Println("TESTING:", opToCheck.SVG, validationMap)
		valCount, ok := validationMap[string(opToCheck.SVGHash.Hash)]
		length := lengthMap[string(opToCheck.SVGHash.Hash)]

		if ok {
			if valCount >= int(opToCheck.ValNum) {
				validatedOpsCH <- opToCheck
				fmt.Println("VAL", valCount, length, opToCheck.ValNum)
			} else if length+valCount < longestChainLength {
				neverValidatedOpsCH <- opToCheck
				fmt.Println("NEVER", valCount, opToCheck.ValNum, length)
			} else {
				operationsToReAdd = append(operationsToReAdd, opToCheck)
				//fmt.Println("OKNO")
			}
		} else {
			operationsToReAdd = append(operationsToReAdd, opToCheck)
//			fmt.Println("NO")
		}
	}
	for _, op := range operationsToReAdd {
		toValidateOpsCH <- op
	}
}

func (m2m *MinerToMiner) ReceiveBlock(block *Block, reply *bool) (err error) {
	if len(block.Ops) > 0 {
		fmt.Println("Received", block.Nonce, block2hash(block))
	}
//	fmt.Println(block.PrevHash, block.Nonce, block.Ops, block.MinedBy)
	var remoteBlock Block
	remoteBlock = *block
	difficulty := ink.settings.PoWDifficultyNoOpBlock
	if len(block.Ops) != 0 {
		difficulty = ink.settings.PoWDifficultyOpBlock
	}
	if validateNonce(block, difficulty) { // TODO: Only nonce?
		//		fmt.Println("trying to validate")
		hash := block2hash(&remoteBlock)
		if debugLocks {
			fmt.Println("locking8")
		}
		maplock.RLock()
		if debugLocks {
			fmt.Println("locked8")
		}
		_, exists := blocks[hash]
		if debugLocks {
			fmt.Println("unlocking8")
		}
		maplock.RUnlock()
		if debugLocks {
			fmt.Println("unlocked8")
		}
		if !exists {
			//			log.Printf("validated nonce = %s from block = %s", remoteBlock.Nonce, hash)
			newBlockCH <- remoteBlock
			foundBlockCH <- remoteBlock // reflood block

			//			m2m.checkValidationOps()
		}
	}
	return
}

func (m2m *MinerToMiner) ReceiveOp(op Operation, reply *bool) (err error) {
//	fmt.Println("OPRECEIVED:", op.SVGHash.Hash)
	newOpCH <- op
	//does not have to put into tovalidate as we only keep track of our own
	return nil
	//does what receiveBlock does for ops
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
	fmt.Println("localaddr: ", ink.localAddr)
	m := &MinerInfo{
		Address: ink.localAddr,
		Key:     ink.key.PublicKey,
	}
	var settings MinerNetSettings
	err = ink.serverClient.Call("RServer.Register", m, &settings)
	if checkError(err) {
		return
	}
	log.Println(settings)

	ink.settings = settings
	return
}

// GetNodes makes RPC GetNodes(pubKey) call, makes a call to ConnectToNeighbour for each returned addr, can return errors
func (m2s *MinerToServer) GetNodes() (err error) {
	if debugLocks { log.Println("neighbourlock6 locking") }
	neighbourlock.Lock()
	if debugLocks { log.Println("neighbourlock6 locked") }


	minerAddresses := make([]net.Addr, 0)
	err = ink.serverClient.Call("RServer.GetNodes", ink.key.PublicKey, &minerAddresses)
	fmt.Println("mineraddrs: ", minerAddresses)
	for _, addr := range minerAddresses {
		_, ok := ink.neighbours[addr.String()]
		if !ok {
			client, err := rpc.Dial("tcp", addr.String())
			if err == nil {
				ink.neighbours[addr.String()] = client
				fmt.Println("added neighbour")
			} else {
				fmt.Println(err)
			}
		}
	}

	if debugLocks { log.Println("neighbourlock6 unlocking") }
	neighbourlock.Unlock()
	if debugLocks { log.Println("neighbourlock6 unlocked") }

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

func (ink IMiner) GetBlockChain() (err error) {
    if debugLocks { fmt.Println("locking9") }
	maplock.Lock()
    if debugLocks { fmt.Println("locked9") }


	if debugLocks { log.Println("neighbourlock7 locking") }
	neighbourlock.RLock()
	if debugLocks { log.Println("neighbourlock7 locked") }
	for _, neighbour := range ink.neighbours {
		blockChain := make([]Block, 0)
		err = neighbour.Call("MinerToMiner.FetchBlockChain", "", &blockChain)
		for _, block := range blockChain {
			blocks[block2hash(&block)] = block
		}
	}
	if debugLocks { log.Println("neighbourlock7 unlocking") }
	neighbourlock.RUnlock()
	if debugLocks { log.Println("neighbourlock7 unlocked") }
	if debugLocks { fmt.Println("unlocking9") }
	maplock.Unlock()
	if debugLocks { fmt.Println("unlocked9") }

	return
}

func (ink IMiner) ProcessNewBlock(b *Block, currentBlock *Block, opQueue []Operation) {
    if debugLocks { fmt.Println("locking10") }
	maplock.Lock()
    if debugLocks { fmt.Println("locked10") }
	blocks[block2hash(b)] = *b
	if debugLocks { fmt.Println("unlocking10") }
	maplock.Unlock()
	if debugLocks { fmt.Println("unlocked10") }
	longestChainHash := ink.getLongestChain()
//	fmt.Println("Switching from:", ink.Length(currentBlock.PrevHash), "to:", ink.Length(longestChainHash))
	for _, o := range currentBlock.Ops {
		isMined := false
		for _, mined := range b.Ops {
			if bytes.Equal(o.SVGHash.Hash, mined.SVGHash.Hash) {
				isMined = true
			}
		}
		if !isMined {
			opQueue = append(opQueue, o)
		}
	}
	*currentBlock = Block{
		PrevHash: longestChainHash,
		MinedBy:  ink.key.PublicKey,
		Ops:      opQueue,
	}
}

func (ink IMiner) ProcessNewOp(op Operation, currentBlock *Block, opQueue []Operation) {
	exists := false

	for _, o := range opQueue {
		if bytes.Equal(op.SVGHash.Hash, o.SVGHash.Hash) {
			exists = true
//			fmt.Println("EXISTS in queue")
		}
	}
	for _, o := range currentBlock.Ops {
		if bytes.Equal(op.SVGHash.Hash, o.SVGHash.Hash) {
			exists = true
//			fmt.Println("EXISTS in block")
		}
	}
	if !exists {
		var newOps []Operation
		newOps = append(currentBlock.Ops, opQueue...)
		if validateOpsForBlock(append(newOps, op), *currentBlock) {
			currentBlock.Ops = append(newOps, op)
		} else {
			currentBlock.Ops = newOps
		}
		var m2m MinerToMiner
		m2m.FloodOpToPeers(op)
	}

	//This is how the miner actually handles the new op when it is handed it through a channel by an RPC call from art node OR a flood from neighbour
}

func (ink IMiner) ProcessMinedBlock(currentBlock *Block, opQueue []Operation) {
	prevHash := block2hash(currentBlock)
	//log.Printf("found nonce = %s with hash = %s", currentBlock.Nonce, prevHash)
    if debugLocks { fmt.Println("locking") }
	maplock.Lock()
    if debugLocks { fmt.Println("locked") }
	blocks[prevHash] = *currentBlock
	if debugLocks { fmt.Println("unlocking") }
	maplock.Unlock()
	if debugLocks { fmt.Println("unlocked") }
	fmt.Println("MINED")
	foundBlockCH <- *currentBlock // spit out the found block via channel
	*currentBlock = Block{
		PrevHash: prevHash,
		MinedBy:  ink.key.PublicKey,
		Ops:      opQueue,
	}
//	var m2m MinerToMiner
//	m2m.checkValidationOps()
}

func (ink IMiner) Mine() (err error) {
	var i uint64 = 0
	opQueue := make([]Operation, 0)

	var currentBlock Block

	if len(blocks) == 0 {
		currentBlock = getGenesisBlock()
	} else {
		// mine the longest chain
		currentBlock = blocks[ink.getLongestChain()]
	}
	go func() {
		for {
			select {
			case b := <-newBlockCH:
				ink.ProcessNewBlock(&b, &currentBlock, opQueue)
				opQueue = make([]Operation, 0)
				i = 0

			case o := <-newOpCH:
				ink.ProcessNewOp(o, &currentBlock, opQueue)
				opQueue = make([]Operation, 0)
				i = 0

			default:
				i++
				if i % 10000 == 0 {
					fmt.Println(i, currentBlock.PrevHash, block2hash(&currentBlock), len(currentBlock.Ops))
				}
				difficulty := ink.settings.PoWDifficultyNoOpBlock
				if len(currentBlock.Ops) != 0 {
					difficulty = ink.settings.PoWDifficultyOpBlock
				}
				currentBlock.Nonce = strconv.FormatUint(i, 10)
				if validateBlock(&currentBlock, difficulty) {
					ink.ProcessMinedBlock(&currentBlock, opQueue)
					opQueue = make([]Operation, 0)
					i = 0
				}
			}
		}
	}()
	return nil
}

func (ink IMiner) GetGenesisBlock() (genesis Block) {
    if debugLocks { fmt.Println("locking2") }
	maplock.RLock()
    if debugLocks { fmt.Println("locked2") }
	genesisBlock := blocks[ink.settings.GenesisBlockHash]
	if debugLocks { fmt.Println("unlocking2") }
	maplock.RUnlock()
	if debugLocks { fmt.Println("unlocked2") }
	return genesisBlock
}

func (ink IMiner) GetChildren(hash string) (children []Block) {
    if debugLocks { fmt.Println("locking3") }
	maplock.RLock()
    if debugLocks { fmt.Println("locked3") }
	children = make([]Block, 0)
	for _, block := range blocks {
		if block.PrevHash == hash {
			children = append(children, block)
		}
	}

	if debugLocks { fmt.Println("unlocking3") }
	maplock.RUnlock()
	if debugLocks { fmt.Println("unlocked3") }
	return
}

func (ink IMiner) getBlockChainHeads() (heads []Block) {
    if debugLocks { fmt.Println("locking4") }
	maplock.RLock()
    if debugLocks { fmt.Println("locked4") }
	possibilities := make(map[string]Block)
	for k, v := range blocks {
		possibilities[k] = v
	}
	for _, block := range blocks {
		delete(possibilities, block.PrevHash)
	}
	if debugLocks { fmt.Println("unlocking4") }
	maplock.RUnlock()
	if debugLocks { fmt.Println("unlocked4") }
	for _, v := range possibilities {
		heads = append(heads, v)
	}

	return
}

func (ink IMiner) Length(hash string) (len int) {
	return ink.LengthFromTo(hash, ink.settings.GenesisBlockHash)
}

func (ink IMiner) LengthFromTo(from string, to string) (length int) {
    if debugLocks { fmt.Println("locking11") }
	maplock.RLock()
    if debugLocks { fmt.Println("locked11") }
	for from != to {
		length += 1
		block, ok := blocks[from]
		if !ok {
			length = 0
			break
		}
		from = block.PrevHash
	}
	if debugLocks { fmt.Println("unlocking11") }
	maplock.RUnlock()
	if debugLocks { fmt.Println("unlocked11") }
	return
}

func (ink IMiner) getLongestChain() (hash string){
	longest := 0
	hash = ink.settings.GenesisBlockHash // hash of the genesis block
	for _, head := range ink.getBlockChainHeads() {
		bhash := block2hash(&head)
		length := ink.Length(bhash)
		if length > longest {
			longest = length
			hash = bhash
		} else if length == longest {
			// equal length chains: pick the larger hash
			if bhash > hash {
				hash = bhash
			}
		}
	}
	return
}

func (ink IMiner) ValidationCount(hash string) (validationCount int) {
	for _, head := range ink.getBlockChainHeads() {
		headLength := ink.LengthFromTo(block2hash(&head), hash)
		if headLength > validationCount {
			validationCount = headLength
		}
	}
	return

}

var ink IMiner
var miner2server MinerToServer
var miner2miner MinerToMiner

var blocks map[string]Block

var newOpCH (chan Operation)
var newBlockCH (chan Block)
var foundBlockCH (chan Block)
var foundOpCH (chan Operation)
var validatedOpsCH (chan Operation)
var toValidateOpsCH (chan Operation)
var neverValidatedOpsCH (chan Operation)

var maplock sync.RWMutex
var neighbourlock sync.RWMutex


func listenForMinerToMinerRPC() net.Addr {
	server := rpc.NewServer()
	server.Register(&MinerToMiner{})

	l, _ := net.Listen("tcp", ":0")

	go func() {
		for {
			conn, _ := l.Accept()
			go server.ServeConn(conn)
		}
	}()

	return l.Addr()
}

func registerGobAndCreateChannels() {
	gob.Register(&net.TCPAddr{})
	gob.Register(&elliptic.CurveParams{})
	gob.Register(&MinerInfo{})
	gob.Register(&[]Block{})
	gob.Register(ecdsa.PrivateKey{})
	gob.Register(Operation{})

	blocks = make(map[string]Block)

	newOpCH = make(chan Operation, math.MaxUint8)
	// big enough to handle one op from each miner
	foundOpCH = make(chan Operation, math.MaxUint16)
	newBlockCH = make(chan Block, math.MaxUint8)
	foundBlockCH = make(chan Block, math.MaxUint8)
	validatedOpsCH = make(chan Operation, math.MaxUint8)
	toValidateOpsCH = make(chan Operation, math.MaxUint8)
	neverValidatedOpsCH = make(chan Operation, math.MaxUint8)

}

func openRPCToServer() (client *rpc.Client, err error) {
	ipPort := flag.String("i", "127.0.0.1:12345", "RPC server ip:port")
	return rpc.Dial("tcp", *ipPort)
}

func getGenesisBlock() (Block) {
	return Block{
		PrevHash: ink.settings.GenesisBlockHash,
		Nonce:    "1337",
		MinedBy:  ecdsa.PublicKey{},
	}
}

func main() {
	registerGobAndCreateChannels()
	server, err := openRPCToServer()
	checkError(err)

	priv, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		fmt.Println(err)
	}

	ink = IMiner{
		serverClient: server,
		key:          *priv,
		localAddr:    listenForMinerToMinerRPC(),
		neighbours:   make(map[string]*rpc.Client),
	}

	// Register with server
	miner2server.Register()

	checkError(miner2server.GetNodes())
	checkError(ink.GetBlockChain())

	// Starts the flood routine that floods new blocks
	startFloodListener()

	// Starts the routine that waits for validated ops
	startValidationListener()

	// Heartbeat server
	heartbeatTheServer()

	// Start mining
	checkError(ink.Mine())

	// Listen incoming RPC calls from artnodes
	go listenForArtNodes()
	defer clearMinerKeyFile()

	// Heartbeat your neighbours s.t. you know when you get some.
	checkError(miner2miner.HeartbeatNeighbours())


	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	for _ = range c {
		dumpBlockchain()
		clearMinerKeyFile()
		os.Exit(0)
	} // This is blocking. Do not add anything after this.
}

func startValidationListener() {
	go func() {
		for {
			var m2m MinerToMiner
			m2m.checkValidationOps()
			time.Sleep(time.Second)
		}
	}()
}

func startFloodListener() {
	go func() {
		for {
			minedBlock := <-foundBlockCH
			miner2miner.FloodBlockToPeers(&minedBlock)
		}
	}()
}

func heartbeatTheServer() {
	go func() {
		for {
			miner2server.HeartbeatServer()
			time.Sleep(time.Millisecond * 5)
		}
	}()
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////
//								END OF ?, START OF ART2MINER IMLEMENTATION											 //
///////////////////////////////////////////////////////////////////////////////////////////////////////////////

type RMiner int

func (m *RMiner) OpenCanvas(keyHash [16]byte, reply *CanvasSettings) error {

	fmt.Println("New ArtNode connecting")

	if hashPrivateKey(ink.key) != keyHash {
		return errors.New("Miner: The key you are connecting with is not correct")
	}

	*reply = ink.settings.CanvasSettings // <-- should have queried the server first
	//*reply = CanvasSettings{1024, 1024} // <-- for now..

	return nil
}

func (m *RMiner) ReceiveNewOp(op Operation, reply *string) error {
	if !ecdsa.Verify(&ink.key.PublicKey, op.SVGHash.Hash, op.SVGHash.R, op.SVGHash.S) {
		return errors.New("Invalid signature")
	}

	newOpCH <- op
	toValidateOpsCH <- op

	for {
		select {
		case validatedOp := <- validatedOpsCH:
			if bytes.Equal(validatedOp.SVGHash.Hash, op.SVGHash.Hash) {
				return nil
			} else {
				validatedOpsCH <- validatedOp
			}
		case neverValidatedOp := <- neverValidatedOpsCH:
			if bytes.Equal(neverValidatedOp.SVGHash.Hash, op.SVGHash.Hash) {
				fmt.Println("READDDED OP")
				newOpCH <- op
				toValidateOpsCH <- op
			} else {
				neverValidatedOpsCH <- neverValidatedOp
			}
		}
	}
	fmt.Println("Added op:", op.SVG)
	return nil
}

func (m *RMiner) getInkForBlock(publicKey ecdsa.PublicKey, block Block, blockHash string) uint32 {
	if block.MinedBy == ink.key.PublicKey {
		allOpsValidated := true
		validateNum := uint8(ink.ValidationCount(blockHash))
		for _, op := range block.Ops {
			if ecdsa.Verify(&publicKey, op.SVGHash.Hash, op.SVGHash.R, op.SVGHash.S) && validateNum < op.ValNum {
				allOpsValidated = false
			}
		}
		if !allOpsValidated {
			return 0
		}
		if len(block.Ops) > 0 {
			return ink.settings.InkPerOpBlock
		} else {
			return ink.settings.InkPerNoOpBlock
		}
	}
	return 0
}

func (m *RMiner) useInkForBlock(publicKey ecdsa.PublicKey, block Block) uint32 {
	var total uint32 = 0
	p := blockartlib.NewSVGParser()
	for _, op := range block.Ops {
		if !ecdsa.Verify(&publicKey, op.SVGHash.Hash, op.SVGHash.R, op.SVGHash.S) {
			continue
		}
		var err error
		if op.Delete {
			op, err = GetOperation(op.SVG)
		}
		shape, err := p.ParseXMLString(op.SVG)
		if checkError(err) {
			continue
		}
		if op.Delete {
			total += shape.Area()
		} else {
			total -= shape.Area()
		}
	}
	return total
}

func (m *RMiner) GetInk(publicKey ecdsa.PublicKey) uint32 {
	longestChainHash := ink.getLongestChain()
	var inkLeft uint32 = 0
	for longestChainHash != ink.settings.GenesisBlockHash {
		if debugLocks { log.Println("locking18") }
		maplock.RLock()
		if debugLocks { log.Println("locked18") }
		b := blocks[longestChainHash]
		if debugLocks { log.Println("unlocking18") }
		maplock.RUnlock()
		if debugLocks { log.Println("unlocked18") }

		inkLeft += m.getInkForBlock(publicKey, b, longestChainHash)
		inkLeft += m.useInkForBlock(publicKey, b)

		longestChainHash = b.PrevHash
	}
	return inkLeft
}

func (m *RMiner) Ink(publicKey ecdsa.PublicKey, reply *uint32) error {
	*reply = m.GetInk(publicKey)
	return nil
}

func GetOperation(shapeHash string) (op Operation, err error) {
	if debugLocks { log.Println("locking19") }
	maplock.RLock()
	if debugLocks { log.Println("locked19") }
	defer maplock.RUnlock()
	for _, block := range blocks {
		for _, op := range block.Ops {
			if string(op.SVGHash.Hash) == shapeHash {
				return op, nil
			}
		}
	}
	return Operation{}, nil //TODO: error
}

func (m *RMiner) GetSVG(shapeHash string, reply *string) (err error) {
	op, err := GetOperation(shapeHash)
	*reply = op.SVG
	return
}

func (m *RMiner) GetShapes(blockHash string, shapeHashes *[]string) error {
	if debugLocks { log.Println("locking20") }
	maplock.RLock()
	if debugLocks { log.Println("locked20") }
	defer maplock.RUnlock()
	for _, op := range blocks[blockHash].Ops {
		*shapeHashes = append(*shapeHashes, string(op.SVGHash.Hash))
	}
	return nil
}

func (m *RMiner) GetGenesisBlock(_unused string, reply *string) error {
	*reply = ink.settings.GenesisBlockHash
	return nil
}
func (m *RMiner) GetChildren(blockHash string, blockHashes *[]string) error {
	for _, child := range ink.GetChildren(blockHash) {
		*blockHashes = append(*blockHashes, block2hash(&child))
	}
	return nil
}
func listenForArtNodes() (err error) {
	fmt.Println("Blocks:", len(blocks))
	checkError(err)

	artServer := rpc.NewServer()
	rminer := new(RMiner)
	artServer.Register(rminer)
	l, err := net.Listen("tcp", ":0") // get address from global ink

	artNodeRPCAddr := l.Addr().String()
	ink.artAddr = artNodeRPCAddr
	writeMinerAddrKeyToFile()

	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Artserver started. Receiving on %s\n", ink.artAddr)
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		go artServer.ServeConn(conn)
	}
}

func hashPrivateKey(key ecdsa.PrivateKey) [16]byte {
	keyBytes, _ := x509.MarshalECPrivateKey(&key)
	return md5.Sum(keyBytes)
}

func decodeKey(hexStr string) (key *ecdsa.PrivateKey, err error) {
	keyBytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return key, err

	}
	return x509.ParseECPrivateKey(keyBytes)
}

func encodeKey(key ecdsa.PrivateKey) (string, error) {
	keyBytes, err := x509.MarshalECPrivateKey(&key)
	if err != nil {
		return "", err
	}
	keyString := hex.EncodeToString(keyBytes)
	return keyString, nil
}

func writeMinerAddrKeyToFile() {
	err := os.MkdirAll("keys/", 0777)
	checkError(err)
	keyString, err := encodeKey(ink.key)
	checkError(err)
	splitAddr := strings.Split(ink.artAddr, ":")
	filename := "./keys/" + splitAddr[len(splitAddr) - 1]
	fmt.Println("Writing address", ink.artAddr, "to file")
	err = ioutil.WriteFile(filename, []byte(keyString), 0777)
	checkError(err)
}

func clearMinerKeyFile() {
	fmt.Println("Deleting file")
	filename := "./keys/" + ink.artAddr
	os.Remove(filename)
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////
//								END OF ART2MINER, START OF VALIDATION										 	 //
///////////////////////////////////////////////////////////////////////////////////////////////////////////////

func block2hash(block *Block) string {
	hasher := md5.New()
	hasher.Write([]byte(block2string(block)))
	return hex.EncodeToString(hasher.Sum(nil))
}

func block2string(block *Block) string {
	//	res, _ := json.Marshal(block.Ops)
	var buffer bytes.Buffer
	for _, op := range block.Ops {
		buffer.Write(op.SVGHash.Hash)
		opString := op.Owner.X.String() + op.Owner.Y.String() + op.SVG
		buffer.WriteString(opString)
	}
	return block.PrevHash + block.Nonce + block.MinedBy.X.String() + block.MinedBy.Y.String() + buffer.String()
}

/*
	Block validations:
		Check that the nonce for the block is valid: PoW is correct and has the right difficulty.
		Check that each operation in the block has a valid signature (this signature should be generated using the private key and the operation).
		Check that the previous block hash points to a legal, previously generated, block.
*/
func validateBlock(block *Block, difficulty uint8) bool {
	validNonce := validateNonce(block, difficulty)
	validPrevHash := validatePrevHash(block)
	validOps := validateOpsForBlock(block.Ops, *block)
	return (validNonce && validOps && validPrevHash)
}

func validateNonce(block *Block, difficulty uint8) bool {
	return strings.HasSuffix(block2hash(block), strings.Repeat("0", int(difficulty)))
}

func validatePrevHash(block *Block) (ok bool) {
	return true
	if block.PrevHash == ink.settings.GenesisBlockHash { // TODO: Handle the special case in a smoother way?
		return true
	}
	if debugLocks { fmt.Println("locking prevhash") }
//	maplock.RLock()
	_, ok = blocks[block.PrevHash]
	if debugLocks { fmt.Println("unlocking ph") }
//	maplock.RUnlock()
	return
}

/*
	Operation validations:
		Check that each operation has sufficient ink associated with the public key that generated the operation.
		Check that each operation does not violate the shape intersection policy described above.
		Check that the operation with an identical signature has not been previously added to the blockchain (prevents operation replay attacks).
		Check that an operation that deletes a shape refers to a shape that exists and which has not been previously deleted.
*/

func validateOpsForBlock(ops []Operation, block Block) bool {
	for _, op := range ops {
		if !validateOp(op, block) {
			return false
		}
	}
	return true
}

func validateOp(op Operation, block Block) bool {
	valInk := true
	valDelete := true
	valIntersection := validateOpIntersection(op, block)
	valSigs := validateOpSigs(op, block)
	if op.Delete {
		valDelete = validateOpDelete(op, block)
	} else {
		valInk = validateOpInk(op, block)
	}
	return valInk && valIntersection && valDelete && valSigs
}

func validateOpInk(op Operation, block Block) bool {
	var rm RMiner
	inkLeft := rm.GetInk(ink.key.PublicKey)
	p := blockartlib.NewSVGParser()
	shape, err := p.ParseXMLString(op.SVG)
	if checkError(err) {
		return true
	}
	return shape.Area() <= inkLeft
}

func validateOpIntersection(op Operation, block Block) bool {
	var ok bool
	p := blockartlib.NewSVGParser()
	shape, _ := p.ParseXMLString(op.SVG)

	for block.PrevHash != ink.settings.GenesisBlockHash {
		for _, o := range block.Ops {
			if !bytes.Equal(o.SVGHash.Hash, op.SVGHash.Hash) && !o.Delete {
				testShape, _ := p.ParseXMLString(o.SVG)
				if blockartlib.Intersects(shape, testShape) && !opIsDeleted(op) {
					return false
				}
			}
		}
		block, ok = blocks[block.PrevHash]
		if !ok {
			return false
		}
	}
	return true
}

func opIsDeleted(op Operation) bool {
	var ok bool
	tipOfChain := blocks[ink.getLongestChain()]
	for tipOfChain.PrevHash != ink.settings.GenesisBlockHash {
		for _, o := range tipOfChain.Ops {
			if o.Delete && o.SVG == string(op.SVGHash.Hash) {
				return true
			}
		}
		tipOfChain, ok = blocks[tipOfChain.PrevHash]
		if !ok {
			return false
		}
	}
	return false
}

func validateOpSigs(op Operation, block Block) bool {
	var ok bool
	block = blocks[block.PrevHash]
	for block.PrevHash != ink.settings.GenesisBlockHash {
		for _, o := range block.Ops {
			if bytes.Equal(op.SVGHash.Hash, o.SVGHash.Hash) {
				return false
			}
		}
		block, ok = blocks[block.PrevHash]
		if !ok {
			return false
		}
	}
	return true
}

func validateOpDelete(op Operation, block Block) bool {
	var ok bool
	for block.PrevHash != ink.settings.GenesisBlockHash {
		for _, o := range block.Ops {
			if !o.Delete && hex.EncodeToString(o.SVGHash.Hash) == op.SVG {
				//fmt.Println("FOUND EQUIVALENT:", hex.EncodeToString(o.SVGHash.Hash), "in", block.PrevHash)
				return true
			}
		}
		block, ok = blocks[block.PrevHash]
		if !ok {
			return false
		}
	}
	return false
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////
//								END OF MINING, START OF UTILITIES											 //
///////////////////////////////////////////////////////////////////////////////////////////////////////////////

func checkError(err error) bool {
	if err != nil {
		fmt.Println("ERROR:", err)
		return true
	}
	return false
}

type DumpStruct struct {
	Hash string
	PrevHash string
	Level int
	Ops int
	OpType string
}

//writes out block's nonce and prevhash and level
func dumpBlockchain() {
	var toDump []DumpStruct
	fmt.Println("BlockChain visualizer 2000")
	fmt.Println("in format of {[blockHash][prevHash][level][|ops|][op-type]}")
	for hash, block := range blocks {
		level := ink.LengthFromTo(hash, ink.settings.GenesisBlockHash)
		opType := "ADD"
		if len(block.Ops) > 0 {
			if block.Ops[0].Delete {
				opType = "DEL"
			}
		} else {
			opType = ""
		}
		thisDump := DumpStruct { hash, block.PrevHash, level, len(block.Ops), opType}
		toDump = append(toDump, thisDump)
		//fmt.Printf("{[%v][%v][%v]}\n", hash, block.PrevHash, level)
	}

	sort.Slice(toDump, func(i, j int) bool { return toDump[i].Level < toDump[j].Level })

	for _, ds := range toDump {
		fmt.Printf("{[%v][%v][%v][%v][%v]}\n", ds.Hash, ds.PrevHash, ds.Level, ds.Ops, ds.OpType)
	}
}
