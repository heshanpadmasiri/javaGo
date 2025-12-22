package converted

type TestData interface {
}

type Test interface {
	TestData
	Process() (string, error)
}

type TestBase struct {
}

type TestMethods struct {
	Self Test
}
