package converted

type FooData interface {
	GetA() int
	SetA(a int)
}

type Foo interface {
	FooData
	F() int
	B() int
}

type FooBase struct {
	A int
}

type FooMethods struct {
	Self Foo
}

type Bar struct {
	FooBase
	FooMethods
}

func newBar() Bar {
	this := Bar{}
	return this
}

func (b *FooBase) GetA() int {
	return b.A
}

func (b *FooBase) SetA(a int) {
	b.A = a
}

func (m *FooMethods) B() int {
	return (m.Self.F() + m.Self.GetA())
}

func (b *Bar) F() int {
	return 42
}
