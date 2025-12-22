package converted

type TestData interface {
}

type Test interface {
	TestData
	Calculate() int
}

type TestBase struct {
}

type TestMethods struct {
	Self Test
}
