package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) test() {
	// migrated from if_else_if_without_final_else.java:2:5
	if x == 1 {
		this.one()
	} else if x == 2 {
		this.two()
	}
}
