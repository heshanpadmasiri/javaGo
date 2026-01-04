package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) test() {
	// migrated from array_initializer_with_string_literals.java:2:5
	names := []string{"Alice", "Bob", "Charlie"}
}
