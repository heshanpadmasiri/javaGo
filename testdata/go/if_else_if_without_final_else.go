package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) test() {
	if x == 1 {
		this.one()
	} else if x == 2 {
		this.two()
	}
}
