package converted

type Drawable interface {
	Draw()
}

type Circle struct {
	radius int
}

var _ Drawable = &Circle{}

func NewCircleFromInt(radius int) Circle {
	this := Circle{}
	this.radius = radius
	return this
}

func (this *Circle) Draw() {
	// migrated from class_implementing_interface.java:12:5
	System.out.println(("Drawing circle with radius " + radius))
}
