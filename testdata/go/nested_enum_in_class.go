package converted

type Inner uint

type Outer struct {
}

const (
	Inner_FIRST Inner = iota
	Inner_SECOND
)

func NewOuter() Outer {
	this := Outer{}
	return this
}
