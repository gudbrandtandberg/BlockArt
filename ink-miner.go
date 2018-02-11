/*

An ink miner mines ink and disseminates blocks

Usage:
go run ink-miner.go [server ip:port] [pubKey] [privKey]

*/

package main

import (
	//"./blockartlib"
	"bytes"
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
	"math/big"
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
	FloodToPeers(block *Block) error
	HeartbeatNeighbours() error
	GetHeartbeats() error
	ReceiveBlock(block *Block, reply *bool) (err error)
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
	// or []byte ??

	// Just a disconnected error? other errors will be handled by methods called within mine
	GetLongestChain() (Block, error)
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

type MinerToMiner struct{}
type MinerToServer struct{}
type MinerFromANode struct{}
type IMiner struct {
	serverClient *rpc.Client
	localAddr    net.Addr
	neighbours   map[string]*rpc.Client

	settings MinerNetSettings

	tails        []*Block
	currentBlock *Block

	key ecdsa.PrivateKey
}

type Operation struct {
	Delete  bool
	SVG     string
	SVGHash SVGHash
	Owner   ecdsa.PublicKey
}

type SVGHash struct {
	Hash []byte
	R, S *big.Int
}

type BlockNode struct {
	Block    Block
	Children []BlockNode
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
	*hash = ink.settings.GenesisBlockHash
	return
}

type InvalidBlockHashError string

func (e InvalidBlockHashError) Error() string {
	return fmt.Sprintf("BlockArt: Invalid block hash [%s]", string(e))
}

func (art MinerFromANode) GetChildren(hash string, childrenHashes *[]string) (err error) {
	nodesToCheck := make([]BlockNode, 0, math.MaxUint32)
	nodesToCheck = append(nodesToCheck, genesisNode)
	for len(nodesToCheck) > 0 {
		var node BlockNode = nodesToCheck[0]
		if block2hash(&node.Block) == hash {
			*childrenHashes = make([]string, 0, len(node.Children))
			for _, child := range node.Children {
				*childrenHashes = append(*childrenHashes, block2hash(&child.Block))
			}
			return
		}
		nodesToCheck = append(nodesToCheck, node.Children...)
	}
	return InvalidBlockHashError(hash)
}

func (m2m *MinerToMiner) FloodToPeers(block *Block) (err error) {
	fmt.Println("Sent", block.Nonce, block2hash(block))

	for _, neighbour := range ink.neighbours {
		var reply bool
		err = neighbour.Call("MinerToMiner.ReceiveBlock", &block, &reply)
		fmt.Println(err, reply)
	}
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
		fmt.Println(len(ink.neighbours), ink.settings.MinNumMinerConnections, ink.neighbours)
		if len(ink.neighbours) >= int(ink.settings.MinNumMinerConnections) {
			return
		}
		//else we get more neighbours
		err = miner2server.GetNodes()
		checkError(err)
	}
}

func (m2m *MinerToMiner) ReceiveBlock(block *Block, reply *bool) (err error) {
	fmt.Println("Received", block.Nonce, block2hash(block))

	difficulty := ink.settings.PoWDifficultyNoOpBlock
	if len(block.Ops) != 0 {
		difficulty = ink.settings.PoWDifficultyOpBlock
	}
	if validateBlock(block, difficulty) {
		fmt.Println("trying to validate")
		for _, head := range ink.getBlockChainHeads() {
			if block2hash(&head.Block) == block.PrevHash {
				fmt.Println("validated")
				newBlockCH <- *block
			} else {
				log.Println("tsk tsk Received block does not append to a head")
			}
		}
	}
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
	minerAddresses := make([]net.Addr, 0)
	err = ink.serverClient.Call("RServer.GetNodes", ink.key.PublicKey, &minerAddresses)
	fmt.Println(minerAddresses)
	for _, addr := range minerAddresses {
		_, ok := ink.neighbours[addr.String()]
		if !ok {
			client, err := rpc.Dial("tcp", addr.String())
			if err == nil {
				ink.neighbours[addr.String()] = client
			} else {
				fmt.Println(err)
			}
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

func (ink IMiner) GetLongestChain() (block *Block, err error) {
	return block, err
}

func (ink IMiner) Mine() (err error) {
	var i uint64 = 0
	opQueue := make([]Operation, 0)
	difficulty := ink.settings.PoWDifficultyNoOpBlock

	var currentBlock Block

	/*
	   type Operation struct {
	   	Svg string
	   	SvgHash SVGHash
	   	Owner ecdsa.PublicKey
	   }
	*/
	go func() {
		for {
			select {
			case b := <-newBlockCH:
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

			case o := <-newOpsCH:
				opQueue = append(opQueue, o)

			default:
				i++
				currentBlock.Nonce = strconv.FormatUint(i, 10)
				if validateBlock(&currentBlock, difficulty) {
					// successfully found nonce
					log.Printf("found nonce: %s", currentBlock.Nonce)
					foundBlockCH <- currentBlock // spit out the found block via channel
					currentBlock = Block{PrevHash: block2hash(&currentBlock), MinedBy: ink.key.PublicKey}
					i = 0
				}
			}
		}
	}()
	return nil
}

func (ink IMiner) getBlockChainHeads() (heads []BlockNode) {
	heads = make([]BlockNode, 0)
	nodesToCheck := make([]BlockNode, 0)
	nodesToCheck = append(nodesToCheck, genesisNode)
	for len(nodesToCheck) > 0 {
		var node BlockNode = nodesToCheck[0]
		nodesToCheck = nodesToCheck[1:]
		if len(node.Children) == 0 {
			heads = append(heads, node)
		} else {
			nodesToCheck = append(nodesToCheck, node.Children...)
		}
	}
	return
}

var ink IMiner
var miner2server MinerToServer
var miner2miner MinerToMiner
var blocks map[string]Block
var bufferedOps []Operation

var newOpsCH (chan Operation)
var newBlockCH (chan Block)
var foundBlockCH (chan Block)

var genesisNode BlockNode

func tmp() net.Addr {
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

func main() {
	gob.Register(&net.TCPAddr{})
	gob.Register(&elliptic.CurveParams{})
	gob.Register(&MinerInfo{})

	newOpsCH = make(chan Operation)
	newBlockCH = make(chan Block)
	foundBlockCH = make(chan Block)

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
		neighbours:   make(map[string]*rpc.Client),
	}

	// Register with server
	miner2server.Register()
	err = miner2server.GetNodes()
	checkError(err)

	genesisBlock := Block{
		PrevHash: ink.settings.GenesisBlockHash,
		MinedBy:  priv.PublicKey,
	}

	go func() {
		for {
			minedBlock := <-foundBlockCH
			miner2miner.FloodToPeers(&minedBlock)
		}
	}()
	ink.currentBlock = &genesisBlock

	fmt.Println(err, ink.neighbours)

	// Heartbeat server
	go func() {
		for {
			miner2server.HeartbeatServer()
			time.Sleep(time.Millisecond * 50)
		}
	}()
	miner2miner.HeartbeatNeighbours()
	checkError(err)

	err = ink.Mine()
	newBlockCH <- genesisBlock

	for {
		time.Sleep(time.Second * 2)
	}

	//genesisNode.Children = append(genesisNode.Children, newNode)
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////
//								END OF METHODS, START OF VALIDATION										 	 //
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
*/
func validateBlock(block *Block, difficulty uint8) bool {
	validNonce := validateNonce(block, difficulty)
	validOps := validateOps(block)
	validPrevHash := validatePrevHash(block)
	return (validNonce && validOps && validPrevHash)
}

func validateNonce(block *Block, difficulty uint8) bool {
	return strings.HasSuffix(block2hash(block), strings.Repeat("0", int(difficulty)))
}

func validatePrevHash(block *Block) bool {
	_, ok := blocks[block.PrevHash]
	if ok {
		return true
	}
	return false
}

/*
Operation validations:
	Check that each operation has sufficient ink associated with the public key that generated the operation.
	Check that each operation does not violate the shape intersection policy described above.
	Check that the operation with an identical signature has not been previously added to the blockchain (prevents operation replay attacks).
	Check that an operation that deletes a shape refers to a shape that exists and which has not been previously deleted.
*/
func validateOps(block *Block) bool {
	opSignatures := validateOpSigs(block)
	intersections := validateIntersections(block)
	identicalSignatures := validateIdenticalSigs(block)
	validDelete := validateDelete(block)

	return (opSignatures && intersections && identicalSignatures && validDelete)
}

//Returns true if all ops are correctly signed
func validateOpSigs(block *Block) bool {
	allTrue := true
	for _, op := range block.Ops {
		if !ecdsa.Verify(&op.Owner, op.SVGHash.Hash, op.SVGHash.R, op.SVGHash.S) {
			allTrue = false
		}
	}
	return allTrue
}

// TODO 
//Returns true if there are -NOT- any intersections with any shapes already in blockchain
func validateIntersections(block *Block) bool {
	return true
}

//Returns true if there is -NOT- an identical op/opsig in blockchain
func validateIdenticalSigs(block *Block) bool {
	noIdenticalOps := true
	var toCheck []Operation
	var theBlocks []Block
	for _, op := range block.Ops {
		//if there is same op/sig in blockchain, add it to a list to check for deletes
		for _, b := range blocks {
			for _, o := range b.Ops {
				if bytes.Equal(o.SVGHash.Hash, op.SVGHash.Hash) {
					toCheck = append(toCheck, op)
					theBlocks = append(theBlocks, b)
				}
			}
		}
	}
	if len(toCheck) > 0 {
		noIdenticalOps = checkDeletes(toCheck, theBlocks)
	}
	return noIdenticalOps
}

//for each op, check if it is deleted in a later block. If it is, return true
func checkDeletes(ops []Operation, blox []Block) bool {
	tipOfChain, err := ink.GetLongestChain()
	checkError(err)

iLoop:
	for i := 0; i < len(ops); i++ {
		thisOp := ops[i]
		thisBlock := blox[i]
		currBlock := tipOfChain
		//iterate back until thisBlock[i] is hit; if it also doesn't have delete then return false
		for currBlock.PrevHash != thisBlock.PrevHash {
			for _, currOp := range currBlock.Ops {
				if currOp.Delete {
					if bytes.Equal(currOp.SVGHash.Hash, thisOp.SVGHash.Hash) {
						break iLoop
					}
				}
			}
			*currBlock = blocks[currBlock.PrevHash]
		}
		finalBlock := blocks[currBlock.PrevHash]
		for _, currOp := range finalBlock.Ops {
			if currOp.Delete {
				if bytes.Equal(currOp.SVGHash.Hash, thisOp.SVGHash.Hash) {
					break iLoop
				}
			}
		}

		return false
	}

	return true

}


//TODO
//Returns true if shape is in blockchain and not previously deleted and is a delete
func validateDelete(block *Block) bool {
	allPossible := true
	tipOfChain, err := ink.GetLongestChain()
	checkError(err)
	for _, op := range block.Ops {
		if op.Delete {
			//if it's a delete, handle it, if it's not a delete ignore
			shapeHash := op.SVG
			currBlock := tipOfChain
			BlockSelectionLoop:
			for currBlock.PrevHash != ink.settings.GenesisBlockHash {
				for _, o := range currBlock.Ops {
					if hex.EncodeToString(o.SVGHash.Hash) == shapeHash {
						break BlockSelectionLoop
					}
				}
				*currBlock = blocks[currBlock.PrevHash]
			}
			if currBlock.PrevHash == ink.settings.GenesisBlockHash {
				return false
			}
		}
	}
	return allPossible
}

/*
type Operation struct {
	Delete  bool
	SVG     string
	SVGHash SVGHash
	Owner   ecdsa.PublicKey
}

type SVGHash struct {
	Hash []byte
	R, S *big.Int
}


*/

///////////////////////////////////////////////////////////////////////////////////////////////////////////////
//								END OF MINING, START OF UTILITIES											 //
///////////////////////////////////////////////////////////////////////////////////////////////////////////////

func checkError(err error) {
	if err != nil {
		fmt.Println("ERROR:", err)
	}
}
