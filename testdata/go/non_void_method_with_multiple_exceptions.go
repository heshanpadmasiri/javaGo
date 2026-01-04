package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) foo() (int, error) {
	// migrated from non_void_method_with_multiple_exceptions.java:2:5
	return 42
}
