package converted

type test struct {
}

func (this *test) process(ctxs *[]Context) {
	this.System.out.println(ctxs)
}
