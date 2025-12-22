package converted

type Drawable interface {
	Draw()
}

type Circle struct {
	radius int
}

var _ Drawable = &Circle{}

func NewCircleFromRadius(radius int) Circle {
	this := Circle{}
	this.radius = radius
	return this
}

func (this *Circle) Draw() {
	this.System.out.println("Drawing circle with radius " + radius)
}
