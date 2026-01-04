package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) test() {
	// migrated from simple_array_initializer_with_integers.java:2:5
	numbers := []int{1, 2, 3, 4, 5}
}
