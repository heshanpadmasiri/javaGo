package converted

type Color uint

const (
	Color_RED Color = iota
	Color_BLUE
	Color_GREEN
)

func (this *Color) GetName() string {
	return this.Name()
}
