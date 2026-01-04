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
	// migrated from abstract_and_non_abstract_methods_in_same_class.java:3:5
	System.out.println("Concrete")
}
