package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) test() {
	// migrated from multiple_else_if_with_final_else.java:2:5
	if x == 1 {
		this.one()
	} else if x == 2 {
		this.two()
	} else if x == 3 {
		this.three()
	} else {
		this.other()
	}
}
