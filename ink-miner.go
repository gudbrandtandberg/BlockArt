/*

	An ink miner mines ink and disseminates blocks

	Usage:
	go run ink-miner.go [server ip:port] [pubKey] [privKey]

*/

package main

import (
	"./blockartlib"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	"crypto/rand"
	"crypto/x509"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
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
	"strconv"
	"strings"
	"sync"
	"time"
)

///////////////////////////////////////////////////////////////////////////////////////////////////////////////
//											START OF INTERFACES												 //
///////////////////////////////////////////////////////////////////////////////////////////////////////////////

type MinerToMinerInterface interface {
	FloodToPeers(block *Block) error
	HeartbeatNeighbours() error
	GetHeartbeats() error
	GetBlockChain() (err error)
	FetchBlockChain(i string, blockchain *[]Block) (err error)
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
	// Just a disconnected error? other errors will be handled by methods called within mine
	GetLongestChain() (Block, error)

	Length(hash string) (err error)
	ValidationCount(hash string) (err error)
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
	*hash = ink.settings.GenesisBlockHash
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

func (m2m *MinerToMiner) FloodToPeers(block *Block) (err error) {
	fmt.Println("Sent", block.Nonce, block2hash(block))
	fmt.Println(block.PrevHash, block.Nonce, block.Ops, block.MinedBy)
	m2m.HeartbeatNeighbours()

	for _, neighbour := range ink.neighbours {
		var reply bool
		err = neighbour.Call("MinerToMiner.ReceiveBlock", &block, &reply)
		//		fmt.Println(err, reply)
	}
	return
}

func (m2m2 *MinerToMiner) GetHeartbeats(incAddr string, out *string) (err error) {
	neighbourlock.Lock()
	neighbourlock.Unlock()

	_, check := ink.neighbours[incAddr]

	if !check {
		//if neighbour doesn't exist
			client, err := rpc.Dial("tcp", incAddr)
			if err == nil {
				ink.neighbours[incAddr] = client
			} else {
				fmt.Println(err)
			}	
		}

	*out = "hello i'm online"
	return
}

func (m2m *MinerToMiner) FetchBlockChain(i string, blockchain *[]Block) (err error) {
	fmt.Println("locking7")
	maplock.Lock()
	fmt.Println("locked7")
	v := make([]Block, 0, len(blocks))
	for _, value := range blocks {
		v = append(v, value)
	}
	*blockchain = v

	fmt.Println("unlocking7")
	maplock.Unlock()
	fmt.Println("unlocked7")
	return
}

func (m2m *MinerToMiner) HeartbeatNeighbours() (err error) {
	for {
		neighbourlock.Lock()
		defer neighbourlock.Unlock()
		for neighbourAddr, neighbourRPC := range ink.neighbours {
			var rep string
			timeout := make(chan error, 1)
			go func() {
				timeout <- neighbourRPC.Call("MinerToMiner.GetHeartbeats", ink.localAddr.String(), &rep)
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
		fmt.Println("len neighbours, minminers, neighbours: ", len(ink.neighbours), ink.settings.MinNumMinerConnections, ink.neighbours)
		if ((len(ink.neighbours) >= int(ink.settings.MinNumMinerConnections)) || (len(ink.neighbours) == 0)) {
			return
		}
		//else we get more neighbours
		err = miner2server.GetNodes()
		checkError(err)
	}
}

func (m2m *MinerToMiner) ReceiveBlock(block *Block, reply *bool) (err error) {
	//	fmt.Println("Received", block.Nonce, block2hash(block), block2string(block))
	//	fmt.Println(block.PrevHash, block.Nonce, block.Ops, block.MinedBy)

	var remoteBlock Block
	remoteBlock = *block
	difficulty := ink.settings.PoWDifficultyNoOpBlock
	if len(block.Ops) != 0 {
		difficulty = ink.settings.PoWDifficultyOpBlock
	}
	if validateBlock(block, difficulty) {
		fmt.Println("trying to validate", ink.getBlockChainHeads())
		fmt.Println("locking8")
		maplock.Lock()
		fmt.Println("locked8")
		_, ok := blocks[block.PrevHash]
		fmt.Println("unlocking8")
		maplock.Unlock()
		fmt.Println("unlocked8")
		if ok {
			fmt.Println("channel ")
			newBlockCH <- remoteBlock
			fmt.Println("channel2")
			fmt.Println("locking13")
			maplock.Lock()
			fmt.Println("locked13")
			hashes := make([]string, 0)
			for k, _ := range blocks {
				hashes = append(hashes, k)
			}
			fmt.Println("unlocking13")
			maplock.Unlock()
			fmt.Println("unlocked13")
			for _, k := range hashes {
				fmt.Println("k, validationcount, length(k): ", k, ink.ValidationCount(k), ink.Length(k))
			}
		} else {
			//			log.Println("tsk tsk Received block does not append to a head")
		}
	} else {
		//		fmt.Println("Not valid", block.PrevHash, block2hash(block))
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

// Register makes RPC Register(localAddr, pubKey) call, and registers settings returned for canvas or returns error
func (m2s *MinerToServer) Register() (err error) {
	fmt.Println("localaddr: ", ink.localAddr)
	m := &MinerInfo{
		Address: ink.localAddr,
		Key:     ink.key.PublicKey,
	}
	var settings MinerNetSettings
	err = ink.serverClient.Call("RServer.Register", m, &settings)
	if err != nil {
		return nil
	}
	log.Println(settings)

	ink.settings = settings
	return
}

// GetNodes makes RPC GetNodes(pubKey) call, makes a call to ConnectToNeighbour for each returned addr, can return errors
func (m2s *MinerToServer) GetNodes() (err error) {
	neighbourlock.Lock()
	defer neighbourlock.Unlock()
	minerAddresses := make([]net.Addr, 0)
	err = ink.serverClient.Call("RServer.GetNodes", ink.key.PublicKey, &minerAddresses)
	fmt.Println("mineraddrs: ", minerAddresses)
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

func (ink IMiner) GetLongestChain() (block *Block, err error) {
	return block, err
}

func (ink IMiner) GetBlockChain() (err error) {
	fmt.Println("locking9")
	maplock.Lock()
	fmt.Println("locked9")

	neighbourlock.Lock()
	defer neighbourlock.Unlock()
	for _, neighbour := range ink.neighbours {
		blockChain := make([]Block, 0)
		err = neighbour.Call("MinerToMiner.FetchBlockChain", "", &blockChain)
		fmt.Println("nei", err, blockChain)
		for _, block := range blockChain {
			blocks[block2hash(&block)] = block
		}
	}
	fmt.Println("unlocking9")
	maplock.Unlock()
	fmt.Println("unlocked9")
	return
}

func (ink IMiner) Mine() (err error) {
	var i uint64 = 0
	opQueue := make([]Operation, 0)
	difficulty := ink.settings.PoWDifficultyNoOpBlock

	var currentBlock Block

	go func() {
		for {
			select {
			case b := <-newBlockCH:
				blocks[block2hash(&b)] = b

				fmt.Println("locking10")
				maplock.Lock()
				fmt.Println("locked10")
				blocks[block2hash(&b)] = b
				fmt.Println("unlocking10")
				maplock.Unlock()
				fmt.Println("unlocked10")
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
				if i%50000 == 0 {
					fmt.Println("mining:", block2hash(&currentBlock), currentBlock.PrevHash)
				}

				currentBlock.Nonce = strconv.FormatUint(i, 10)
				if validateBlock(&currentBlock, difficulty) {
					// successfully found nonce
					log.Printf("found nonce: %s", currentBlock.Nonce)
					prevHash := block2hash(&currentBlock)
					blocks[prevHash] = currentBlock
					foundBlockCH <- currentBlock // spit out the found block via channel
					//log.Printf("block hash: %s", prevHash)
					fmt.Println("locking")
					maplock.Lock()
					fmt.Println("locked")
					blocks[prevHash] = currentBlock
					fmt.Println("unlocking")
					maplock.Unlock()
					fmt.Println("unlocked")
					foundBlockCH <- currentBlock // spit out the found block via channel
					//log.Println("difficulty: ", ink.settings)
					//					newBlockCH <- currentBlock // spit out the found block via channel
					currentBlock = Block{
						PrevHash: prevHash,
						MinedBy:  ink.key.PublicKey,
						Ops:      opQueue,
					}
					opQueue = make([]Operation, 0)
					i = 0
				}
			}
		}
	}()
	return nil
}

func (ink IMiner) GetGenesisBlock() (genesis Block) {
	return blocks[ink.settings.GenesisBlockHash]
}

func (ink IMiner) GetChildren(hash string) (children []Block) {
	fmt.Println("locking3")
	maplock.Lock()
	fmt.Println("locked3")
	children = make([]Block, 0)
	for _, block := range blocks {
		if block.PrevHash == hash {
			children = append(children, block)
		}
	}

	fmt.Println("unlocking3")
	maplock.Unlock()
	fmt.Println("unlocked3")
	return
}

func (ink IMiner) getBlockChainHeads() (heads []Block) {
	fmt.Println("locking4")
	maplock.Lock()
	fmt.Println("locked4")
	possibilities := make(map[string]Block)
	for k, v := range blocks {
		possibilities[k] = v
	}
	for _, block := range blocks {
		delete(possibilities, block.PrevHash)
	}
	for _, v := range possibilities {
		heads = append(heads, v)
	}

	fmt.Println("unlocking4")
	maplock.Unlock()
	fmt.Println("unlocked4")
	return
}

func (ink IMiner) Length(hash string) (len int) {
	return ink.LengthFromTo(hash, ink.settings.GenesisBlockHash)
}

func (ink IMiner) LengthFromTo(fromHash string, toHash string) (len int) {
	fmt.Println("locking11")
	maplock.Lock()
	fmt.Println("locked11")
	from := fromHash
	to := toHash
	for from != to {
		len += 1
		block, ok := blocks[from]
		if !ok {
			len = 0
			break
		}
		from = block.PrevHash
	}
	fmt.Println("unlocking11")
	maplock.Unlock()
	fmt.Println("unlocked11")
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

var newOpsCH (chan Operation)
var newBlockCH (chan Block)
var foundBlockCH (chan Block)

var maplock sync.RWMutex
var neighbourlock sync.RWMutex


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
	gob.Register(&[]Block{})

	blocks = make(map[string]Block)

	newOpsCH = make(chan Operation, 255)
	newBlockCH = make(chan Block, 255)
	foundBlockCH = make(chan Block, 255)

	priv, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		fmt.Println(err)
	}

	ipPort := flag.String("i", "127.0.0.1:12345", "RPC server ip:port")
	client, err := rpc.Dial("tcp", *ipPort)

	l := tmp()
	ink = IMiner{
		serverClient: client,
		key:          *priv,
		localAddr:    l,
		artAddr:      "", // <-- this is set in listenForArtNodes()
		neighbours:   make(map[string]*rpc.Client),
	}

	// Register with server
	miner2server.Register()
	err = miner2server.GetNodes()
	err = ink.GetBlockChain()

	// Listen incoming RPC calls from artnodes
	go listenForArtNodes()

	genesisBlock := Block{
		PrevHash: "foobar",
		Nonce:    "1337",
		MinedBy:  ecdsa.PublicKey{},
	}

	go func() {
		for {
			minedBlock := <-foundBlockCH
			miner2miner.FloodToPeers(&minedBlock)
		}
	}()

	fmt.Println(err, ink.neighbours)

	// Heartbeat server
	go func() {
		for {
			miner2server.HeartbeatServer()
			//fmt.Println("len of new block: ", len(newBlockCH))
			time.Sleep(time.Millisecond * 5)
		}
	}()
	miner2miner.HeartbeatNeighbours()
	checkError(err)

	err = ink.Mine()
	newBlockCH <- genesisBlock

	c := make(chan os.Signal, 1) 
	signal.Notify(c, os.Interrupt)
	go func() {
		for _ = range c {
			dumpBlockchain()
			clearMinerKeyFile()
			os.Exit(0)
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

func (m *RMiner) RecordDeleteOp(op Operation, reply *string) error {
	fmt.Println("Will delete: ", op.SVG)

	if !ecdsa.Verify(&ink.key.PublicKey, op.SVGHash.Hash, op.SVGHash.R, op.SVGHash.S) {
		return errors.New("Invalid signature")
	}

	return nil
}

func (m *RMiner) RecordAddOp(op Operation, reply *string) error {
	fmt.Println("Will add this shape to my current block:")

	if !ecdsa.Verify(&ink.key.PublicKey, op.SVGHash.Hash, op.SVGHash.R, op.SVGHash.S) {
		return errors.New("Invalid signature")
	}

	fmt.Println("recorded: ", op.SVG)
	return nil
}

func listenForArtNodes() (err error) {
	gob.Register(ecdsa.PrivateKey{})
	gob.Register(&elliptic.CurveParams{})
	gob.Register(Operation{})

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
	fmt.Printf("Artserver started. Receiving on %s\n", ink.localAddr)
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
	keyString, _ := encodeKey(ink.key)
	filename := "./keys/" + ink.artAddr
	fmt.Println("Writing file")
	ioutil.WriteFile(filename, []byte(keyString), 0666)
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
	return block.PrevHash + block.Nonce + block.MinedBy.X.String() + block.MinedBy.Y.String()

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
>>>>>>> 02c4270da860c3cff88e5410d1b8ca5832ea1e01
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
	
	noIntersections := true
	var toCheck []Operation
	var theBlocks []Block

	//for each op in block, for each block in bc, for each op in block of bc, check if xmlstring of op1 intersections with op2
	for _, op := range block.Ops {
		for _, bl := range blocks {
			for _, blOp := range bl.Ops {
				if blockartlib.XMLStringsIntersect(op.SVG, blOp.SVG) {
					//check if intersecting op was deleted later. if it was then it's fine if not return false
					toCheck = append(toCheck, blOp)
					theBlocks = append(theBlocks, bl)
					//if same shape is added and deleted multiple times, this still works
				}
				//else if they do not intersect move on to next op in block of blockchain
			}
		}
	}

	if len(toCheck) > 0 {
		noIntersections = checkDeletes(toCheck, theBlocks)
	}
	return noIntersections
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

///////////////////////////////////////////////////////////////////////////////////////////////////////////////
//								END OF MINING, START OF UTILITIES											 //
///////////////////////////////////////////////////////////////////////////////////////////////////////////////

func checkError(err error) {
	if err != nil {
		fmt.Println("ERROR:", err)
	}
}


//writes out block's nonce and prevhash and level
func dumpBlockchain() {
	fmt.Println("BlockChain visualizer 2000")
	fmt.Println("in format of {[blockHash][prevHash][level]}")
	for hash, block := range blocks {
		level := ink.LengthFromTo(hash, ink.settings.GenesisBlockHash)
		fmt.Printf("{[%v][%v][%v]}\n", hash, block.PrevHash, level)
	}

}