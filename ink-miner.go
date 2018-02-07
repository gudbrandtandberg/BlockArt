/*

An ink miner mines ink and disseminates blocks

Usage:
go run ink-miner.go [server ip:port] [pubKey] [privKey]

*/

package main

import (
	"fmt"
)

///////////////////////////////////////////////////////////////////////////////////////////////////////////////
//											START OF INTERFACES												 //
///////////////////////////////////////////////////////////////////////////////////////////////////////////////

type MinerToMinerInterface interface {
	ConnectToNeighbour() error
	FloodToPeers() error
	HeartbeatNeighbours() error

}

type MinerFromMinerInterface interface {
	RegisterNeighbour() error
	ReceiveAndFlood() error
}

////// TCP RPC calls to make against server
/*
settings, err ← Register(address, publicKey)
	Registers a new miner with an address for other miner to use to connect to it (returned in GetNodes call below)
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
	pubKey string
	privKey string

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











///////////////////////////////////////////////////////////////////////////////////////////////////////////////
//								END OF STRUCTS, START OF METHODS											 //
///////////////////////////////////////////////////////////////////////////////////////////////////////////////

func main() {
	fmt.Prinln("To be implemented..")
}
