package converted

type test struct {
}

func (this *test) combine(numbers ...int) {
	this.System.out.println(numbers)
}
