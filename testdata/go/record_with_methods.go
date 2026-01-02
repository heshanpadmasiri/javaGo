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
	return ((this.X * this.X) + (this.Y * this.Y))
}
