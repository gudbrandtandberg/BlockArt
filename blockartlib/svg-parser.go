package blockartlib

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// EPSILON is the threshold used for checking if a float is zero
const EPSILON = 10E-6

var commands = map[string]int{"M": 2, "m": 2, "L": 2, "l": 2, "H": 1, "h": 1, "V": 1, "v": 1, "z": 0, "Z": 0}
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

// An SVGParser parses SVG shape commands into point-based datastructures
// (need not be a struct as of now, but might be useful later..)
type SVGParser struct {
}

// NewSVGParser is a SVGParser constructor
func NewSVGParser() *SVGParser {
	parser := SVGParser{}
	return &parser
}

// Parse parses an SVG string and returns a list of components
func (p *SVGParser) Parse(svgString string) (Components, error) {
	var reader = strings.NewReader(svgString)
	var arg1, arg2 float64
	var lastPoint Point2d // defaults to the origin
	var components = make(Components, 0)
	var component = make(Component, 0)

	for cmd, err := getCommand(reader); err == nil; cmd, err = getCommand(reader) {
		if !isCommand(cmd) {
			return nil, errors.New("The command " + cmd + " is not a valid SVG command")
		}

		if contains(moveToCmds, cmd) && isClosedPath(component) {
			return components, errors.New("cannot have multiple closed components in same path")
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
				return components, errors.New("cannot have multiple closed components in same path")
			}
			if len(component) < 3 {
				return components, errors.New("cannot close path with fewer than three points")
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
	return components, nil
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

func (l Line2d) length() float64 {
	return math.Sqrt(math.Pow(l.b.x-l.a.x, 2) + math.Pow(l.b.y-l.a.y, 2))
}

// Intersects is true if the two shapes described by the arguments intersect
func Intersects(shape1, shape2 Components) bool {
	// !?!??
	return openIntersects(shape1, shape2)
}

// openIntersects is true if any of the lines in the first shape
// intersect with any of the lines in the second shape
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

func euclidDist(u, v Point2d) float64 {
	return math.Pow(u.x-v.x, 2) + math.Pow(u.y-v.y, 2)
}

func isClosedPath(path Component) bool {
	if len(path) < 3 {
		return false
	}
	return euclidDist(path[len(path)-1], path[0]) < EPSILON
}

func isClosed(c Components) bool {
	if len(c) != 1 {
		return false
	}
	if isClosedPath(c[0]) {
		return true
	}
	return false
}

func isOpen(c Components) bool {
	return !isClosed(c)
}
