package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) test() {
	// migrated from empty_array_initializer.java:2:5
	empty := []int{}
}
