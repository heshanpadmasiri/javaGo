package converted

type Inner uint

type Outer struct {
	X int
}

const (
	Inner_ONE Inner = iota
	Inner_TWO
)

func NewOuter() Outer {
	this := Outer{}
	return this
}
