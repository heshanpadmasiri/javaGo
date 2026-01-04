package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) foo() {
	// migrated from method_without_exceptions_control.java:2:5
	System.out.println("test")
}
