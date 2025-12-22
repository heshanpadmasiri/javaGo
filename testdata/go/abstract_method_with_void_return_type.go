package converted

type TestData interface {
}

type Test interface {
	TestData
	DoSomething()
}

type TestBase struct {
}

type TestMethods struct {
	Self Test
}
