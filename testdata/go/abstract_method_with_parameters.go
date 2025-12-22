package converted

type TestData interface {
}

type Test interface {
	TestData
	Process(input string, count int) string
}

type TestBase struct {
}

type TestMethods struct {
	Self Test
}
