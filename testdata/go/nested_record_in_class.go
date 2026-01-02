package converted

type Inner struct {
	Value int
}

type Outer struct {
}

func NewInner() Inner {
	this := Inner{}
	return this
}

func NewOuter() Outer {
	this := Outer{}
	return this
}
