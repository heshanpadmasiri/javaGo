package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) combine(numbers ...int) {
	// migrated from spread_parameters_should_not_be_wrapped.java:2:5
	System.out.println(numbers)
}
