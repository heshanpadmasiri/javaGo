package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) test() {
	if x > 0 {
		this.positive()
	} else if x < 0 {
		this.negative()
	} else {
		this.zero()
	}
}
