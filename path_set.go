package geo

// A PathSet represents a set of paths in the 2D Eucledian or Cartesian plane.
type PathSet []Path

// NewPathSet simply creates a new path set with an empty paths slice.
func NewPathSet() *PathSet {
	return &PathSet{}
}

// NewPathSetPreallocate simply creates a new path set with paths array of the given size.
func NewPathSetPreallocate(length, capacity int) *PathSet {
	if length > capacity {
		capacity = length
	}

	ps := make([]Path, length, capacity)
	p := PathSet(ps)
	return &p
}

// Clone returns a new copy of the path set.
func (ps PathSet) Clone() *PathSet {
	paths := make([]Path, len(ps))
	copy(paths, ps)

	nps := PathSet(paths)
	return &nps
}

//SetPaths sets the paths in the path set
func (ps *PathSet) SetPaths(paths []Path) *PathSet {
	*ps = paths
	return ps
}
