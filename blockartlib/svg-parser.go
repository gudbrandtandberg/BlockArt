/*
svg-parser.go is part of the blockartlib package and contains functions and
datastructures for dealing with parsing SVG shapes and computing area and
intersection of parsed shapes (TO BE INCLUDED IN blockartlib.go)
*/

package blockartlib

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"html/template"
	"math"
	"strconv"
	"strings"
)

// EPSILON is the threshold used for checking if a float is zero
const EPSILON = 10E-6

var commands = map[string]int{"M": 2, "m": 2, "L": 2, "l": 2,
						"H": 1, "h": 1, "V": 1, "v": 1, "z": 0, "Z": 0}
var moveToCmds = []string{"M", "m", "Z", "z"} //TODO: Z/z should not be "moveTo"..
var lineToCmds = []string{"L", "l", "H", "h", "V", "v"}

// A Point2d represents a point in a 2D Cartesian coordinate system
type Point2d struct {
	x, y float64
}

// String is used when printing points
func (p Point2d) String() string {
	return "(" + strconv.FormatFloat(p.x, 'f', -1, 64) +
		", " + strconv.FormatFloat(p.y, 'f', -1, 64) + ")"
}

// Minus returns the difference of two points
func (p Point2d) Minus(other Point2d) Point2d {
	return Point2d{p.x - other.x, p.y - other.y}
}

// A Line2d represents a line segment from points a to b
type Line2d struct {
	a, b Point2d
}

// A Component is a ordered sequence of points
type Component []Point2d

// A Components object is a set of disjoint components
type Components []Component

// A Shape represents a shape
type Shape interface {
	Area() uint32
	XMLString() string
}

// A PathShape represents a path shape
type PathShape struct {
	d          string
	components Components
	fill       string
	stroke     string
}

// A CircleShape represents a circle shape
type CircleShape struct {
	cx, cy, r float64
	fill      string
	stroke    string
}

func (c CircleShape) center() Point2d {
	return Point2d{c.cx, c.cy}
}

func (c CircleShape) radius() float64 {
	return c.r
}

// Area returns the area of a path element
func (p PathShape) Area() uint32 {
	if p.fill != "transparent" {
		return uint32(math.Ceil(ShapeArea(p.components[0])))
	}
	return uint32(math.Ceil(LineArea(p.components)))
}

// Area returns the area of a circle element
func (c CircleShape) Area() uint32 {
	if c.fill != "transparent" {
		return uint32(math.Ceil(math.Pi * math.Pow(c.r, 2)))
	}
	return uint32(math.Ceil(2 * math.Pi * c.r))
}

var pathTemplate = "<svg><path d='{{.SVGString}}' fill='{{.Fill}}' stroke='{{.Stroke}}'/></svg>"
var circleTemplate = "<svg><circle cx='{{.Cx}}' cy='{{.Cy}}' r='{{.R}}' fill='{{.Fill}}' stroke='{{.Stroke}}'/></svg>"

func (p PathShape) XMLString() string {
	data := struct{ SVGString, Fill, Stroke string }{p.d, p.fill, p.stroke}
	tmpl, _ := template.New("").Parse(pathTemplate)
	var b bytes.Buffer
	_ = tmpl.Execute(&b, data)
	return b.String()
}

func (c CircleShape) XMLString() string {
	cx := strconv.FormatFloat(c.cx, 'f', -1, 64)
	cy := strconv.FormatFloat(c.cy, 'f', -1, 64)
	r := strconv.FormatFloat(c.r, 'f', -1, 64)
	data := struct{ Cx, Cy, R, Fill, Stroke string }{cx, cy, r, c.fill, c.stroke}
	tmpl, err := template.New("").Parse(circleTemplate)
	var b bytes.Buffer
	err = tmpl.Execute(&b, data)
	if err != nil {
		return err.Error()
	}
	return b.String()
}

// An SVGParser parses SVG shape commands into point-based datastructures
// (need not be a struct as of now, but might be useful later..)
type SVGParser struct {
}

// NewSVGParser is a SVGParser constructor
func NewSVGParser() *SVGParser {
	parser := SVGParser{}
	return &parser
}

type SVGXML struct {
	Path   PathXML   `xml:"path"`
	Circle CircleXML `xml:"circle"`
}
type PathXML struct {
	SVGString string `xml:"d,attr"`
	Fill      string `xml:"fill,attr"`
	Stroke    string `xml:"stroke,attr"`
}

type CircleXML struct {
	Cx     string `xml:"cx,attr"`
	Cy     string `xml:"cy,attr"`
	R      string `xml:"r,attr"`
	Fill   string `xml:"fill,attr"`
	Stroke string `xml:"stroke,attr"`
}

func (p *SVGParser) ParseXMLString(XMLString string) (shape Shape, err error) {
	var svg SVGXML
	err = xml.Unmarshal([]byte(XMLString), &svg)
	if err != nil {
		return
	}

	nullPath := PathXML{}
	nullCircle := CircleXML{}

	if svg.Path != nullPath {
		return p.Parse(PATH, svg.Path.SVGString, svg.Path.Fill, svg.Path.Stroke)
	} else if svg.Circle != nullCircle {
		SVGString := svg.Circle.Cx + ", " + svg.Circle.Cy + ", " + svg.Circle.R
		return p.Parse(CIRCLE, SVGString, svg.Circle.Fill, svg.Circle.Stroke)
	}
	fmt.Println("bottomed out, should not be here.. ")
	return
}

// Parse parses an SVG string and returns a list of components
// Can return the following errors
// - InvalidShapeSvgStringError
// - ShapeSvgStringTooLongError
func (p *SVGParser) Parse(shapeType ShapeType, svgString, fill, stroke string) (Shape, error) {

	if len(svgString) > 128 {
		return nil, ShapeSvgStringTooLongError(svgString)
	}
	if shapeType == CIRCLE {
		return parseCircle(svgString, fill, stroke)
	} else if shapeType == PATH {
		return parseShape(svgString, fill, stroke)
	} else if shapeType == STAR {
		return parseShape(svgString, fill, stroke)
	}

	return nil, errors.New("Unknown ShapeType")
}

// Can return the following errors
// - InvalidShapeSvgStringError
func parseCircle(svgString, fill, stroke string) (Shape, error) {
	var reader = strings.NewReader(svgString)
	buffer := make([]byte, 129)
	n, _ := reader.Read(buffer)
	cmd := string(buffer[:n])
	parts := strings.Split(cmd, ",")
	if len(parts) != 3 {
		return nil, InvalidShapeSvgStringError(svgString)
	}
	cx, err := strconv.ParseFloat(strings.Trim(parts[0], " "), 64)
	cy, err := strconv.ParseFloat(strings.Trim(parts[1], " "), 64)
	r, err := strconv.ParseFloat(strings.Trim(parts[2], " "), 64)
	if err != nil {
		return nil, InvalidShapeSvgStringError(svgString)
	}
	return CircleShape{cx, cy, r, fill, stroke}, nil
}

// Can return the following errors
// - InvalidShapeSvgStringError
func parseShape(svgString, fill, stroke string) (Shape, error) {
	var reader = strings.NewReader(svgString)
	var arg1, arg2 float64
	var lastPoint Point2d // defaults to the origin
	var components = make(Components, 0)
	var component = make(Component, 0)

	for cmd, err := getCommand(reader); err == nil; cmd, err = getCommand(reader) {
		if !isCommand(cmd) {
			return nil, InvalidShapeSvgStringError(svgString)
		}

		if contains(moveToCmds, cmd) && isClosedPath(component) {
			fmt.Println("cannot have multiple closed components in same path")
			return nil, InvalidShapeSvgStringError(svgString)
		}

		if contains(moveToCmds, cmd) && len(component) != 0 {
			if cmd == "m" {
				lastPoint = component[len(component)-1]
			}
			if !(cmd == "Z" || cmd == "z") {
				components = append(components, component)
				component = make(Component, 0)
			}
		}

		if contains(lineToCmds, cmd) && len(component) == 0 {
			component = append(component, lastPoint)
		}

		numArgs := commands[cmd]
		switch numArgs {
		case 0: // cmd is z or Z
			if len(components) > 1 {
				fmt.Println("cannot have multiple closed components in same path")
				return nil, InvalidShapeSvgStringError(svgString)
			}
			if len(component) < 3 {
				fmt.Println("cannot close path with fewer than three points")
				return nil, InvalidShapeSvgStringError(svgString)
			}
			if len(component) != 0 {
				component = append(component, component[0])
			}

		case 1: // cmd can be H, V, h, v
			arg1, err = getArg(reader)
			if err != nil {
				return nil, err
			}
			switch cmd {
			case "H":
				component = append(component, Point2d{arg1, lastPoint.y})
			case "V":
				component = append(component, Point2d{lastPoint.x, arg1})
			case "h":
				component = append(component, Point2d{lastPoint.x + arg1, lastPoint.y})
			case "v":
				component = append(component, Point2d{lastPoint.x, lastPoint.y + arg1})
			}

		case 2: // cmd can be M, m, L, l
			arg1, arg2, err = getArgs(reader)
			if err != nil {
				return nil, err
			}
			switch cmd {
			case "M":
				component = append(component, Point2d{arg1, arg2})
			case "m":
				component = append(component, Point2d{lastPoint.x + arg1, lastPoint.y + arg2})
			case "L":
				component = append(component, Point2d{arg1, arg2})
			case "l":
				component = append(component, Point2d{lastPoint.x + arg1, lastPoint.y + arg2})
			}
		}
		if len(component) != 0 {
			lastPoint = component[len(component)-1]
		}
	}
	components = append(components, component)

	return PathShape{svgString, components, fill, stroke}, nil
}

func getCommand(reader *strings.Reader) (string, error) {
	var cmd string
	_, err := fmt.Fscanf(reader, "%s", &cmd)
	if err != nil {
		return "", err
	}
	return cmd, nil
}

func getArg(reader *strings.Reader) (float64, error) {
	var arg float64
	_, err := fmt.Fscanf(reader, "%f", &arg)
	if err != nil {
		return 0.0, err
	}
	return arg, nil
}

func getArgs(reader *strings.Reader) (float64, float64, error) {
	var arg1, arg2 float64
	_, err := fmt.Fscanf(reader, "%f %f", &arg1, &arg2)
	if err != nil {
		return 0.0, 0.0, err
	}
	return arg1, arg2, nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func isCommand(cmd string) bool {
	_, exists := commands[cmd]
	return exists
}

// ShapeArea computes the area of a closed component.
// Works for arbitrary (non-convex, non-self-intersecting) shapes,
// provided 'vertices' describes an oriented, closed path.
func ShapeArea(vertices Component) float64 {
	area := 0.0
	n := len(vertices)
	j := n - 1 // The last vertex is the 'previous' one to the first
	for i := 0; i < n; i++ {
		area += (vertices[j].x + vertices[i].x) * (vertices[j].y - vertices[i].y)
		j = i
	}
	return math.Abs(area / 2)
}

// LineArea computes the sum of the perimeters of each of the components.
// Works for any set of components
func LineArea(components Components) float64 {
	area := 0.0
	for _, line := range components.lines() {
		area += line.length()
	}
	return area
}

func (c Component) lines() (lines []Line2d) {
	for i := 0; i < len(c)-1; i++ {
		lines = append(lines, Line2d{c[i], c[i+1]})
	}
	return lines
}

func (c Components) lines() (lines []Line2d) {
	for _, comp := range c {
		lines = append(lines, comp.lines()...)
	}
	return lines
}

func (c Components) points() (points []Point2d) {
	for _, comp := range c {
		points = append(points, comp...)
	}
	return points
}

func (l Line2d) length() float64 {
	return math.Sqrt(math.Pow(l.b.x-l.a.x, 2) + math.Pow(l.b.y-l.a.y, 2))
}

func (c BACanvas) circleIsOutOfBounds(circ CircleShape) bool {
	x, y := float64(c.settings.CanvasXMax), float64(c.settings.CanvasYMax)
	oo := Point2d{circ.r, circ.r}
	xy := Point2d{x - circ.r, y - circ.r}
	return pointIsOutOfBounds(circ.center(), oo, xy)
}

func (c BACanvas) pathIsOutOfBounds(path PathShape) bool {
	x, y := float64(c.settings.CanvasXMax), float64(c.settings.CanvasYMax)
	oo := Point2d{}
	xy := Point2d{x, y}
	for _, point := range path.components.points() {
		if pointIsOutOfBounds(point, oo, xy) {
			return true
		}
	}
	return false
}

// is the point 'p' outside the rectangle spanned by the two points 'oo' and 'xy'
// assuming oo.x < xy.x and oo.y < xy.y
func pointIsOutOfBounds(p, oo, xy Point2d) bool {
	if (p.x >= oo.x && p.x <= xy.x) && (p.y >= oo.y && p.y <= xy.y) {
		return false
	}
	return true
}

// XMLStringsIntersect checks if two shapes intersect based on their xml representation.
// Ignores errors because there should be NO ILLEGAL XML SHAPE STRINGS in our project!
func XMLStringsIntersect(shapeString1, shapeString2 string) bool {
	parser := NewSVGParser()
	shape1, _ := parser.ParseXMLString(shapeString1)
	shape2, _ := parser.ParseXMLString(shapeString1)
	return Intersects(shape1, shape2)
}

// Intersects is true if the 'shape1' and 'shape2' intersect
func Intersects(shape1, shape2 Shape) bool {

	// Path / Path
	p1, isPath1 := shape1.(PathShape)
	p2, isPath2 := shape2.(PathShape)

	if isPath1 && isPath2 {
		if p1.fill == "transparent" && p2.fill == "transparent" { // Open / Open
			return openIntersects(p1.components, p2.components)
		} else if p1.fill != "transparent" && p2.fill != "transparent" { // Closed / Closed
			if openIntersects(p1.components, p2.components) {
				return true
			}
			for _, point := range p1.components.points() {
				if pointInPolygon(point, p2.components[0]) {
					return true
				}
			}
			for _, point := range p2.components.points() {
				if pointInPolygon(point, p1.components[0]) {
					return true
				}
			}
		} else { // Open / Closed
			if openIntersects(p1.components, p2.components) {
				return true
			}
			if p1.fill == "transparent" && p2.fill != "transparent" {
				for _, point := range p1.components.points() {
					if pointInPolygon(point, p2.components[0]) {
						return true
					}
				}
			}
			if p1.fill != "transparent" && p2.fill == "transparent" {
				for _, point := range p2.components.points() {
					if pointInPolygon(point, p1.components[0]) {
						return true
					}
				}
			}
			return false
		}
	}

	// Circle / Circle
	c1, isCircle1 := shape1.(CircleShape)
	c2, isCircle2 := shape2.(CircleShape)
	if isCircle1 && isCircle2 {
		return circlesIntersect(c1, c2)
	}

	// Circle / Path
	if isPath1 && isCircle2 {
		return pathIntersectsCircle(p1, c2)
	}
	if isCircle1 && isPath2 {
		return pathIntersectsCircle(p2, c1)
	}

	return false
}

// A path shape intersects a circle shape
func pathIntersectsCircle(path PathShape, circle CircleShape) bool {
	if circle.fill != "transparent" {
		// a path intersects a disk if and only if at least one of
		// the points along the path is inside the circle
		for _, point := range path.components.points() {
			if circleContainsPoint(circle, point) {
				return true
			}
		}
	}
	// fill == transparent; check if any of the lines intersect the circle
	for _, line := range path.components.lines() {
		if circleIntersectsLine(circle, line) {
			return true
		}
	}
	return false
}

func circleIntersectsLine(circle CircleShape, line Line2d) bool {
	if circleContainsPoint(circle, line.a) && !circleContainsPoint(circle, line.b) ||
		!circleContainsPoint(circle, line.a) && circleContainsPoint(circle, line.b) {
		return true
	}
	return false
}

func circleContainsPoint(circle CircleShape, point Point2d) bool {
	return euclidDist(point, circle.center()) <= circle.radius()
}

func circlesIntersect(c1, c2 CircleShape) bool {

	delta := euclidDist(c1.center(), c2.center())
	if delta > c1.radius()+c2.radius() {
		return false
	}

	minRad := argmin(c1.radius(), c2.radius()) + 1

	if minRad == 1 { // c1 is smallest
		if delta-c1.radius() <= c2.radius() {
			if c2.fill != "transparent" {
				return true
			}
		}
	} else if minRad == 2 { //c2 is smallest
		if delta-c2.radius() <= c1.radius() {
			if c1.fill != "transparent" {
				return true
			}
		}
	}

	return false
}

func argmin(x ...float64) int {
	min := math.MaxFloat64
	arg := 0
	for i, num := range x {
		if num < min {
			min = num
			arg = i
		}
	}
	return arg
}

// openIntersects is true if any of the lines in the first shape
// intersect with any of the lines in the second shape
// - OpenPath / OpenPath
func openIntersects(shape1, shape2 Components) bool {
	for _, line1 := range shape1.lines() {
		for _, line2 := range shape2.lines() {
			if linesIntersect(line1, line2) {
				return true
			}
		}
	}
	return false
}

// linesIntersect is true if line1 intersects with line2
// Computes the intersection of line1 = a->b and line2 = c->d by
// using the parameterizations
// 		l1(s) = a + (b-a)*s, s in [0, 1]
//		l2(t) = c + (d-c)*t, t in [0, 1]
// and solving the linear system
// 		[(b-a) (c-d)][s t]^T = (c-a)
// to determine where the lines intersect,
// and check if both s and t indeed lie in [0,1]
func linesIntersect(line1, line2 Line2d) bool {
	A1 := line1.b.Minus(line1.a)
	A2 := line2.a.Minus(line2.b)
	rhs := line2.a.Minus(line1.a)
	det := A1.x*A2.y - A1.y*A2.x
	if math.Abs(det) < EPSILON {
		if line1.contains(line2.a) || line1.contains(line2.b) || line2.contains(line1.a) {
			return true
		}
	}
	det1 := rhs.x*A2.y - rhs.y*A2.x
	det2 := A1.x*rhs.y - A1.y*rhs.x
	s := det1 / det
	t := det2 / det
	if 0 <= s && s <= 1.0 && 0 <= t && t <= 1.0 {
		return true
	}
	return false
}

// contains returns true if the point p lies on the line segment l.
// It is asssumed that det(l.a->p, l.a->l.b) == 0.
func (l Line2d) contains(p Point2d) bool {
	a := l.a
	b := l.b
	s := 0.0
	if math.Abs(b.x-a.x) < EPSILON { //line is vertical
		if !(math.Abs(p.x-a.x) < EPSILON) { //point does not have same x-coord as line
			return false
		}
		s = p.y - a.y/b.y - a.y
	} else if math.Abs(b.y-a.y) < EPSILON { //line is horizontal
		if !(math.Abs(p.y-a.y) < EPSILON) { //point does not have same y-coord as line
			return false
		}
		s = p.x - a.x/b.x - a.x
	} else {
		s1 := p.x - a.x/b.x - a.x
		s2 := p.y - a.y/b.y - a.y
		if !(math.Abs(s1-s2) < EPSILON) {
			fmt.Println("s1 and s2 differ. Inputs are not parallel")
			return false
		}
		s = s1
	}
	if 0 <= s && s <= 1.0 {
		return true
	}
	return false
}

// pointInPolygon returns true if 'point' is inside the polygon
// from: http://alienryderflex.com/polygon/
func pointInPolygon(point Point2d, vertices Component) bool {
	numVertices := len(vertices)
	x := point.x
	y := point.y
	i := 0
	j := numVertices - 1
	oddNodes := false
	for i = 0; i < numVertices; i++ {
		if (vertices[i].y < y && vertices[j].y >= y) || (vertices[j].y < y && vertices[i].y >= y) {
			if vertices[j].x+(y-vertices[i].y)/(vertices[j].y-vertices[i].y)*(vertices[j].x-vertices[j].x) < x {
				oddNodes = !oddNodes
			}
		}
		j = i
	}
	return oddNodes
}

func euclidDist(u, v Point2d) float64 {
	return math.Sqrt(math.Pow(u.x-v.x, 2) + math.Pow(u.y-v.y, 2))
}

func isClosedPath(path Component) bool {
	if len(path) < 3 {
		return false
	}
	return euclidDist(path[len(path)-1], path[0]) < EPSILON
}

func (p PathShape) String() string {
	compString := fmt.Sprintf("%s", p.components)
	s := ""
	s += "<Path>\n"
	s += "  Components: " + compString + "\n"
	s += "  Fill: " + p.fill + "\n"
	s += "  Stroke: " + p.stroke + "\n"
	s += "  Area: " + strconv.Itoa(int(p.Area())) + "\n"
	s += "</Path>"
	return s
}

func (c CircleShape) String() string {
	s := ""
	s += "<Circle>\n"
	s += "  Center: (" + strconv.FormatFloat(c.cx, 'f', -1, 64) + ", " + strconv.FormatFloat(c.cy, 'f', -1, 64) + ")\n"
	s += "  Radius: " + strconv.FormatFloat(c.r, 'f', -1, 64) + "\n"
	s += "  Fill: " + c.fill + "\n"
	s += "  Stroke: " + c.stroke + "\n"
	s += "  Area: " + strconv.Itoa(int(c.Area())) + "\n"
	s += "</Circle>"
	return s
}
