package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) checkFlags(flags *[]bool) {
	this.System.out.println(flags)
}
