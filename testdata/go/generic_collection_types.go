package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) processList(items *[]String) {
	this.System.out.println(items)
}
