package converted

type TestData interface {
}

type Test interface {
	TestData
	Foo() error
}

type TestBase struct {
}

type TestMethods struct {
	Self Test
}
