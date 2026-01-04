package converted

type Point struct {
	X int
	Y int
}

func NewPoint() Point {
	this := Point{}
	return this
}

func (this *Point) Distance() int {
	// migrated from record_with_methods.java:2:5
	return ((this.X * this.X) + (this.Y * this.Y))
}
