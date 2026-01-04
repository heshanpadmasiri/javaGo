package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) checkFlags(flags *[]bool) {
	// migrated from boolean_array_parameter.java:2:5
	System.out.println(flags)
}
