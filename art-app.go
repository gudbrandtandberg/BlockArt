/*

A trivial application to illustrate how the blockartlib library can be
used from an application in project 1 for UBC CS 416 2017W2.

From spec:
"an application that uses blockartlib and produces an html file as output that contains
 an svg canvas that is the result of the application's distributed activity"

Usage:
go run art-app.go
*/

package main

// Expects blockartlib.go to be in the ./blockartlib/ dir, relative to
// this art-app.go file
import (
	"fmt"
	"os"

	"./blockartlib"
)

var examples = map[string]string{
	"a":     "M 2 1 h 1 v 1 h 1 v 1 h -1 v 1 h -1 v -1 h -1 v -1 h 1 z",
	"b":     "M 10 10 l 100 45 H 30 z",
	"c":     "M 37 17 v 15 H 14 V 17 z",
	"d":     "M 0 0 L 2 0 V 1 l 1 1 l -1 1 l -2 -2 z",
	"diag1": "M 1 1 L 3 3",
	"diag2": "M 1 3 L 3 1",
	"diag3": "M 3 2 L 4 3",
	"diag4": "M 1 1 L 4 4",
	"cross": "M 1 1 L 3 3 M 1 3 L 3 1",
	"sq1":   "M 1 1 h 2 v 2 h -2 z",
	"sq2":   "M 2 2 h 2 v 2 h -2 z",
	"h1":    "h 1",
	"h2":    "M 0.5 0 h 1",
}

func main() {
	// minerAddr := "127.0.0.1:8080"
	// curve := elliptic.P224()
	// privKey, err := ecdsa.GenerateKey(curve, rand.Reader)

	parser := blockartlib.NewSVGParser()
	c1, err := parser.Parse(examples["h1"])
	if checkError(err) != nil {
		return
	}
	c2, err := parser.Parse(examples["h2"])
	if checkError(err) != nil {
		return
	}
	fmt.Println(c1)
	fmt.Println(c2)

	fmt.Println("Line area:", blockartlib.LineArea(c1))
	fmt.Println("Shape area:", blockartlib.ShapeArea(c1[0]))
	fmt.Println("Intersect:", blockartlib.Intersects(c1, c2))

	// Open a canvas.
	// _, _, err = blockartlib.OpenCanvas(minerAddr, *privKey)
	// if checkError(err) != nil {
	// 	return
	// }

	// validateNum := uint8(2)

	// // Add a line.
	// shapeHash, blockHash, ink, err := canvas.AddShape(validateNum, blockartlib.PATH, "M 0 0 L 0 5", "transparent", "red")
	// if checkError(err) != nil {
	// 	return
	// }

	// // Add another line.
	// shapeHash2, blockHash2, ink2, err := canvas.AddShape(validateNum, blockartlib.PATH, "M 0 0 L 5 0", "transparent", "blue")
	// if checkError(err) != nil {
	// 	return
	// }

	// // Delete the first line.
	// ink3, err := canvas.DeleteShape(validateNum, shapeHash)
	// if checkError(err) != nil {
	// 	return
	// }

	// // assert ink3 > ink2

	// // Close the canvas.
	// ink4, err := canvas.CloseCanvas()
	// if checkError(err) != nil {
	// 	return
	// }
}

// If error is non-nil, print it out and return it.
func checkError(err error) error {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error ", err.Error())
		return err
	}
	return nil
}
