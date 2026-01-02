package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) test() {
	names := []string{"Alice", "Bob", "Charlie"}
}
