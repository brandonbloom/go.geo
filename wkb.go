package geo

import (
	"errors"
	"math"
)

var (
	// ErrUnsupportedDataType is returned by Scan methods when asked to scan
	// non []byte data from the database. This should never happen
	// if the driver is acting appropriately.
	ErrUnsupportedDataType = errors.New("go.geo: scan value must be []byte")

	// ErrNotWKB is returned when unmarshalling WKB and the data is not valid.
	ErrNotWKB = errors.New("go.geo: invalid WKB data")

	// ErrIncorrectGeometry is returned when unmarshalling WKB data into the wrong type.
	// For example, unmarshaling linestring data into a point.
	ErrIncorrectGeometry = errors.New("go.geo: incorrect geometry")
)

// NewPointFromWKB will take raw WKB and set the data for a new point.
// The WKB data must be of type Point. Will return nil if invalid WKB point.
func NewPointFromWKB(wkb []byte) *Point {
	p := &Point{}
	if err := p.unmarshalWKB(wkb); err != nil {
		return nil
	}

	return p
}

// NewLineFromWKB will take raw WKB and set the data for a new line.
// The WKB data must of type LineString and only contain 2 points.
// Will return nil if invalid WKB.
func NewLineFromWKB(wkb []byte) *Line {
	l := &Line{}
	if err := l.unmarshalWKB(wkb); err != nil {
		return nil
	}

	return l
}

// NewPointSetFromWKB will take raw WKB and set the data for a new point set.
// The WKB data must be of type LineString, Polygon or MultiPoint.
// Will return nil if invalid WKB.
func NewPointSetFromWKB(wkb []byte) *PointSet {
	ps := &PointSet{}
	if err := ps.unmarshalWKB(wkb); err != nil {
		return nil
	}

	return ps
}

// NewPathFromWKB will take raw WKB and set the data for a new path.
// The WKB data must be of type LineString, Polygon or MultiPoint.
// Will return nil if invalid WKB.
func NewPathFromWKB(wkb []byte) *Path {
	p := NewPath()
	if err := p.PointSet.unmarshalWKB(wkb); err != nil {
		return nil
	}

	return p
}

// NewPathSetFromWKB will take raw WKB and set the data for a new path set.
// The WKB data must be of type LineString, Polygon, MultiPoint,
// MultiLineString, or MultiPolygon.
// Will return nil if invalid WKB.
func NewPathSetFromWKB(wkb []byte) *PathSet {
	p := NewPathSet()
	if err := p.unmarshalWKB(wkb); err != nil {
		return nil
	}

	return p
}

// Scan implements the sql.Scanner interface allowing
// point structs to be passed into rows.Scan(...interface{})
// The column must be of type Point and must be fetched in WKB format.
// Will attempt to parse MySQL's SRID+WKB format if the data is of the right size.
// If the column is empty (not null) an empty point (0, 0) will be returned.
func (p *Point) Scan(value interface{}) error {
	data, ok := value.([]byte)
	if !ok {
		return ErrUnsupportedDataType
	}

	if len(data) < 21 {
		// the length of a point type in WKB
		return p.unmarshalWKB(data)
	}

	return ErrIncorrectGeometry
}

func (p *Point) unmarshalWKB(data []byte) error {
	if len(data) < 21 {
		return ErrNotWKB
	}

	littleEndian, typeCode, err := scanPrefix(data)
	if err != nil {
		return err
	}

	if typeCode != 1 {
		return ErrIncorrectGeometry
	}

	p[0] = scanFloat64(data[5:13], littleEndian)
	p[1] = scanFloat64(data[13:21], littleEndian)

	return nil
}

// Scan implements the sql.Scanner interface allowing
// line structs to be passed into rows.Scan(...interface{})
// The column must be of type LineString and contain 2 points,
// or an error will be returned. Data must be fetched in WKB format.
// Will attempt to parse MySQL's SRID+WKB format if the data is of the right size.
// If the column is empty (not null) an empty line [(0, 0), (0, 0)] will be returned.
func (l *Line) Scan(value interface{}) error {
	data, ok := value.([]byte)
	if !ok {
		return ErrUnsupportedDataType
	}

	if len(data) < 41 {
		// the length of a 2 point linestring type in WKB
		return l.unmarshalWKB(data)
	}

	return ErrIncorrectGeometry
}

func (l *Line) unmarshalWKB(data []byte) error {
	if len(data) < 41 {
		return ErrNotWKB
	}

	littleEndian, typeCode, err := scanPrefix(data)
	if err != nil {
		return err
	}

	if typeCode != 2 {
		return ErrIncorrectGeometry
	}

	length := scanUint32(data[5:9], littleEndian)
	if length != 2 {
		return ErrIncorrectGeometry
	}

	l.a[0] = scanFloat64(data[9:17], littleEndian)
	l.a[1] = scanFloat64(data[17:25], littleEndian)
	l.b[0] = scanFloat64(data[25:33], littleEndian)
	l.b[1] = scanFloat64(data[33:41], littleEndian)

	return nil
}

// Scan implements the sql.Scanner interface allowing
// line structs to be passed into rows.Scan(...interface{})
// The column must be of type LineString, Polygon or MultiPoint
// or an error will be returned. Data must be fetched in WKB format.
// Will attempt to parse MySQL's SRID+WKB format if obviously no WKB
// or parsing as WKB fails.
// If the column is empty (not null) an empty point set will be returned.
func (ps *PointSet) Scan(value interface{}) error {
	data, ok := value.([]byte)
	if !ok {
		return ErrUnsupportedDataType
	}

	if len(data) < 6 {
		return ErrNotWKB
	}

	// first byte of real WKB data indicates endian and should 1 or 0.
	if data[0] == 0 || data[0] == 1 {
		return ps.unmarshalWKB(data)
	}

	return ps.unmarshalWKB(data[4:])
}

func (ps *PointSet) unmarshalWKB(data []byte) error {
	_, err := scanPoints(ps, data)
	return err
}

// Returns unread data and maybe an error.
func scanPoints(ps *PointSet, data []byte) ([]byte, error) {
	if len(data) < 6 {
		return data, ErrNotWKB
	}

	littleEndian, typeCode, err := scanPrefix(data)
	if err != nil {
		return data, err
	}

	// must be LineString, Polygon or MultiPoint
	if typeCode != 2 && typeCode != 3 && typeCode != 4 {
		return data, ErrIncorrectGeometry
	}

	if typeCode == 3 {
		// For polygons there is a ring count.
		// We only allow one ring here.
		rings := int(scanUint32(data[5:9], littleEndian))
		if rings != 1 {
			return data, ErrIncorrectGeometry
		}

		data = data[9:]
	} else {
		data = data[5:]
	}

	length := int(scanUint32(data[:4], littleEndian))
	if len(data) < 4+16*length {
		return data, ErrNotWKB
	}
	data = data[4:]

	points := make([]Point, length, length)
	for i := 0; i < length; i++ {
		points[i][0] = scanFloat64(data[:8], littleEndian)
		data = data[8:]
		points[i][1] = scanFloat64(data[:8], littleEndian)
		data = data[8:]
	}

	ps.SetPoints(points)
	return data, nil
}

// Scan implements the sql.Scanner interface allowing
// line structs to be passed into rows.Scan(...interface{})
// The column must be of type LineString, Polygon or MultiPoint
// or an error will be returned. Data must be fetched in WKB format.
// Will attempt to parse MySQL's SRID+WKB format if obviously no WKB
// or parsing as WKB fails.
// If the column is empty (not null) an empty path will be returned.
func (p *Path) Scan(value interface{}) error {
	return p.PointSet.Scan(value)
}

// Scan implements the sql.Scanner interface allowing
// slices of path structs to be passed into rows.Scan(...interface{})
// The column must be of type MultiLineString, MultiPolygon, LineString,
// Polygon or MultiPoint, otherwise an error will be returned.
// Data must be fetched in WKB format.
// Will attempt to parse MySQL's SRID+WKB format if obviously no WKB
// or parsing as WKB fails.
// If the column is empty (not null) an empty point set will be returned.
func (ps *PathSet) Scan(value interface{}) error {
	data, ok := value.([]byte)
	if !ok {
		return ErrUnsupportedDataType
	}

	if len(data) < 6 {
		return ErrNotWKB
	}

	// first byte of real WKB data indicates endian and should 1 or 0.
	if data[0] == 0 || data[0] == 1 {
		return ps.unmarshalWKB(data)
	}

	return ps.unmarshalWKB(data[4:])
}

func (ps *PathSet) unmarshalWKB(data []byte) error {
	if len(data) < 6 {
		return ErrNotWKB
	}

	littleEndian, typeCode, err := scanPrefix(data)
	if err != nil {
		return err
	}

	// Promote LineString, Polygon or MultiPoint to a set.
	if typeCode == 2 || typeCode == 3 || typeCode == 4 {
		path := NewPath()
		err := path.unmarshalWKB(data)
		ps.SetPaths([]Path{*path})
		return err
	}

	// must be MultiLineString or MultiPolygon
	if typeCode != 5 && typeCode != 6 {
		return ErrIncorrectGeometry
	}

	// Get path count.
	length := int(scanUint32(data[5:9], littleEndian))
	data = data[9:]

	paths := make([]Path, length, length)
	for i := 0; i < length; i++ {
		points := NewPointSet()
		data, err = scanPoints(points, data)
		if err != nil {
			return err
		}
		paths[i] = Path{*points}
	}

	ps.SetPaths(paths)

	return nil
}

func scanPrefix(data []byte) (bool, uint32, error) {
	if len(data) < 6 {
		return false, 0, ErrNotWKB
	}

	if data[0] == 0 {
		return false, scanUint32(data[1:5], false), nil
	}

	if data[0] == 1 {
		return true, scanUint32(data[1:5], true), nil
	}

	return false, 0, ErrNotWKB
}

func scanUint32(data []byte, littleEndian bool) uint32 {
	var v uint32

	if littleEndian {
		for i := 3; i >= 0; i-- {
			v <<= 8
			v |= uint32(data[i])
		}
	} else {
		for i := 0; i < 4; i++ {
			v <<= 8
			v |= uint32(data[i])
		}
	}

	return v
}

func scanFloat64(data []byte, littleEndian bool) float64 {
	var v uint64

	if littleEndian {
		for i := 7; i >= 0; i-- {
			v <<= 8
			v |= uint64(data[i])
		}
	} else {
		for i := 0; i < 8; i++ {
			v <<= 8
			v |= uint64(data[i])
		}
	}

	return math.Float64frombits(v)
}
