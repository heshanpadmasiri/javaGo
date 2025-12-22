package converted

type test struct {
}

func (this *test) checkFlags(flags *[]bool) {
	this.System.out.println(flags)
}
