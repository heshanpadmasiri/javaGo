package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) foo() (string, error) {
	// migrated from non_void_method_with_single_exception.java:2:5
	return "test"
}
