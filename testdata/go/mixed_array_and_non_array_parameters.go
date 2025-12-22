package converted

type test struct {
}

func (this *test) process(count int, data *[]int, name string) {
	this.System.out.println(count)
}
