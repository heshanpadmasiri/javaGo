package converted

type TestData interface {
}

type Test interface {
	TestData
	AbstractMethod()
	ConcreteMethod()
}

type TestBase struct {
}

type TestMethods struct {
	Self Test
}

func (m *TestMethods) ConcreteMethod() {
	System.out.println("Concrete")
}
