package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) compare(arr1 *[]int, arr2 *[]string) {
	this.System.out.println("Comparing")
}
