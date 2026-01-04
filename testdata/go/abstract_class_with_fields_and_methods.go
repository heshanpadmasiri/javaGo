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

func (b *FooBase) GetA() int {
	return b.A
}

func (b *FooBase) SetA(a int) {
	b.A = a
}

func (m *FooMethods) B() int {
	// migrated from abstract_class_with_fields_and_methods.java:4:5
	return (m.Self.F() + m.Self.GetA())
}
