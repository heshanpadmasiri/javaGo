package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) foo() (int, error) {
	return 42
}
