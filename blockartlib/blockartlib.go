/*
This package specifies the application's interface to the the BlockArt
library (blockartlib) to be used in project 1 of UBC CS 416 2017W2.
*/

package blockartlib

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	"crypto/rand"
	"crypto/x509"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/rpc"
)

// ShapeType represents a type of shape in the BlockArt system.
type ShapeType int

// ShapeType constants
const (
	PATH ShapeType = iota
	CIRCLE
)

// CanvasSettings contain settings for a canvas in BlockArt.
type CanvasSettings struct {
	// Canvas dimensions
	CanvasXMax uint32
	CanvasYMax uint32
}

// MinerNetSettings contain settings for an instance of the BlockArt project/network.
type MinerNetSettings struct {
	// Hash of the very first (empty) block in the chain.
	GenesisBlockHash string

	// The minimum number of ink miners that an ink miner should be
	// connected to. If the ink miner dips below this number, then
	// they have to retrieve more nodes from the server using
	// GetNodes().
	MinNumMinerConnections uint8

	// Mining ink reward per op and no-op blocks (>= 1)
	InkPerOpBlock   uint32
	InkPerNoOpBlock uint32

	// Number of milliseconds between heartbeat messages to the server.
	HeartBeat uint32

	// Proof of work difficulty: number of zeroes in prefix (>=0)
	PoWDifficultyOpBlock   uint8
	PoWDifficultyNoOpBlock uint8

	// Canvas settings
	canvasSettings CanvasSettings
}

////////////////////////////////////////////////////////////////////////////////////////////
// <ERROR DEFINITIONS>

// These type definitions allow the application to explicitly check
// for the kind of error that occurred. Each API call below lists the
// errors that it is allowed to raise.
//
// Also see:
// https://blog.golang.org/error-handling-and-go
// https://blog.golang.org/errors-are-values

// DisconnectedError address IP:port that art node cannot connect to.
type DisconnectedError string

func (e DisconnectedError) Error() string {
	return fmt.Sprintf("BlockArt: cannot connect to [%s]", string(e))
}

// InsufficientInkError contains amount of ink remaining.
type InsufficientInkError uint32

func (e InsufficientInkError) Error() string {
	return fmt.Sprintf("BlockArt: Not enough ink to addShape [%d]", uint32(e))
}

// InvalidShapeSvgStringError contains the offending svg string.
type InvalidShapeSvgStringError string

func (e InvalidShapeSvgStringError) Error() string {
	return fmt.Sprintf("BlockArt: Bad shape svg string [%s]", string(e))
}

// ShapeSvgStringTooLongError contains the offending svg string.
type ShapeSvgStringTooLongError string

func (e ShapeSvgStringTooLongError) Error() string {
	return fmt.Sprintf("BlockArt: Shape svg string too long [%s]", string(e))
}

// InvalidShapeHashError contains the bad shape hash string.
type InvalidShapeHashError string

func (e InvalidShapeHashError) Error() string {
	return fmt.Sprintf("BlockArt: Invalid shape hash [%s]", string(e))
}

// ShapeOwnerError contains the bad shape hash string.
type ShapeOwnerError string

func (e ShapeOwnerError) Error() string {
	return fmt.Sprintf("BlockArt: Shape owned by someone else [%s]", string(e))
}

// OutOfBoundsError is empty
type OutOfBoundsError struct{}

func (e OutOfBoundsError) Error() string {
	return fmt.Sprintf("BlockArt: Shape is outside the bounds of the canvas")
}

// ShapeOverlapError contains the hash of the shape that this shape overlaps with.
type ShapeOverlapError string

func (e ShapeOverlapError) Error() string {
	return fmt.Sprintf("BlockArt: Shape overlaps with a previously added shape [%s]", string(e))
}

// InvalidBlockHashError contains the invalid block hash.
type InvalidBlockHashError string

func (e InvalidBlockHashError) Error() string {
	return fmt.Sprintf("BlockArt: Invalid block hash [%s]", string(e))
}

// </ERROR DEFINITIONS>
////////////////////////////////////////////////////////////////////////////////////////////

////////////////////////////////////////////////////////////////////////////////////////////
// <ART NODE 2 MINER IMPLEMENTATION>

var minerClient *rpc.Client

// </ART NOTE 2 MINER IMPLEMENTATION>
////////////////////////////////////////////////////////////////////////////////////////////

////////////////////////////////////////////////////////////////////////////////////////////
// <CANVAS IMPLEMENTATION>

// A Canvas represents a canvas in the system.
type Canvas interface {
	AddShape(validateNum uint8, shapeType ShapeType, shapeSvgString string, fill string, stroke string) (shapeHash string, blockHash string, inkRemaining uint32, err error)
	GetSvgString(shapeHash string) (svgString string, err error)
	GetInk() (inkRemaining uint32, err error)
	DeleteShape(validateNum uint8, shapeHash string) (inkRemaining uint32, err error)
	GetShapes(blockHash string) (shapeHashes []string, err error)
	GetGenesisBlock() (blockHash string, err error)
	GetChildren(blockHash string) (blockHashes []string, err error)
	CloseCanvas() (inkRemaining uint32, err error)
}

// OpenCanvas is the constructor for a new Canvas object instance. Takes the miner's
// IP:port address string and a public-private key pair (ecdsa private
// key type contains the public key). Returns a Canvas instance that
// can be used for all future interactions with blockartlib.
//
// The returned Canvas instance is a singleton: an application is
// expected to interact with just one Canvas instance at a time.
//
// Can return the following errors:
// - DisconnectedError
func OpenCanvas(minerAddr string, privKey ecdsa.PrivateKey) (canvas Canvas, setting CanvasSettings, err error) {
	gob.Register(ecdsa.PrivateKey{})
	gob.Register(&elliptic.CurveParams{})
	gob.Register(Operation{})

	client, err := rpc.Dial("tcp", minerAddr)
	if err != nil {
		err = DisconnectedError(minerAddr)
		return
	}
	minerClient = client

	keyHash := hashPrivateKey(privKey)

	err = minerClient.Call("RMiner.OpenCanvas", keyHash, &setting)
	if err != nil {
		return
	}

	canvas = BACanvas{setting, privKey}

	return canvas, setting, nil
}

//////////////////////////////
// BACanvas implementation. //
//////////////////////////////

// The BACanvas object implements the Canvas interface.
type BACanvas struct {
	settings CanvasSettings
	privKey  ecdsa.PrivateKey
}

// AddShape adds a new shape to the canvas.
// Can return the following errors:
// - DisconnectedError
// - InsufficientInkError
// - InvalidShapeSvgStringError
// - ShapeSvgStringTooLongError
// - ShapeOverlapError
// - OutOfBoundsError
func (c BACanvas) AddShape(validateNum uint8, shapeType ShapeType, shapeSvgString string, fill string, stroke string) (shapeHash string, blockHash string, inkRemaining uint32, err error) {

	parser := NewSVGParser()
	shape, err := parser.Parse(shapeType, shapeSvgString, fill, stroke)

	if err != nil {
		return
	}

	if path, ok := shape.(PathShape); ok {
		fmt.Println("Attempting to add a path element:")
		fmt.Println(path)
		if c.pathIsOutOfBounds(path) {
			err = OutOfBoundsError{}
			return
		}

	}
	if circ, ok := shape.(CircleShape); ok {
		fmt.Println("Attemting to add a circle element:")
		fmt.Println(circ)
		if c.circleIsOutOfBounds(circ) {
			err = OutOfBoundsError{}
			return
		}
	}

	// local checks done, send op to miner
	var op Operation
	op.Delete = false
	op.Owner = c.privKey.PublicKey
	op.SVG = shape.XMLString()
	h := md5.New()
	shHash := h.Sum([]byte(op.Svg))
	r, s, err := ecdsa.Sign(rand.Reader, &c.privKey, shHash)
	op.SvgHash = SVGHash{shHash, r, s}
	shapeHash = string(shHash)

	err = minerClient.Call("RMiner.AddShape", op, nil)
	if err != nil {
		return
	}

	fmt.Println("everything went well")
	return
}

// GetSvgString returns the encoding of the shape as an svg string.
// Can return the following errors:
// - DisconnectedError
// - InvalidShapeHashError
func (c BACanvas) GetSvgString(shapeHash string) (svgString string, err error) {
	return
}

// GetInk returns the amount of ink currently available.
// Can return the following errors:
// - DisconnectedError
func (c BACanvas) GetInk() (inkRemaining uint32, err error) {
	return
}

// DeleteShape removes a shape from the canvas.
// Can return the following errors:
// - DisconnectedError
// - ShapeOwnerError
func (c BACanvas) DeleteShape(validateNum uint8, shapeHash string) (inkRemaining uint32, err error) {
	return
}

// GetShapes retrieves hashes contained by a specific block.
// Can return the following errors:
// - DisconnectedError
// - InvalidBlockHashError
func (c BACanvas) GetShapes(blockHash string) (shapeHashes []string, err error) {
	return
}

// GetGenesisBlock returns the block hash of the genesis block.
// Can return the following errors:
// - DisconnectedError
func (c BACanvas) GetGenesisBlock() (blockHash string, err error) {
	return
}

// GetChildren retrieves the children blocks of the block identified by blockHash.
// Can return the following errors:
// - DisconnectedError
// - InvalidBlockHashError
func (c BACanvas) GetChildren(blockHash string) (blockHashes []string, err error) {
	return
}

// CloseCanvas closes the canvas/connection to the BlockArt network.
// - DisconnectedError
func (c BACanvas) CloseCanvas() (inkRemaining uint32, err error) {
	return
}

// </CANVAS IMPLEMENTATION>
////////////////////////////////////////////////////////////////////////////////////////////

////////////////////////////////////////////////////////////////////////////////////////////
// <EXTRA STUFF>

func hashPrivateKey(key ecdsa.PrivateKey) [16]byte {
	keyBytes, _ := x509.MarshalECPrivateKey(&key)
	return md5.Sum(keyBytes)
}

func encodeKey(key ecdsa.PrivateKey) (string, error) {
	keyBytes, err := x509.MarshalECPrivateKey(&key)
	if err != nil {
		return "", err
	}
	keyString := hex.EncodeToString(keyBytes)
	return keyString, nil
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

// </EXTRA STUFF>
////////////////////////////////////////////////////////////////////////////////////////////
