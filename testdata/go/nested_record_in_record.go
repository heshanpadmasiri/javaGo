package converted

type Inner struct {
	Y int
}

type Outer struct {
	X int
}

func NewInner() Inner {
	this := Inner{}
	return this
}

func NewOuter() Outer {
	this := Outer{}
	return this
}
