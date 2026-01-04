package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) foo() error {
	// migrated from void_method_with_single_exception.java:2:5
	System.out.println("test")
}
