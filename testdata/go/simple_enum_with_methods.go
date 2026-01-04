package converted

type Color uint

const (
	Color_RED Color = iota
	Color_BLUE
	Color_GREEN
)

func (this *Color) GetName() string {
	// migrated from simple_enum_with_methods.java:6:5
	return this.Name()
}
