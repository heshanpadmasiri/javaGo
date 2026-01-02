package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) foo() (string, error) {
	return "test"
}
