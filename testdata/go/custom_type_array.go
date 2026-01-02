package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) process(ctxs *[]Context) {
	this.System.out.println(ctxs)
}
