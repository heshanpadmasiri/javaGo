package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) process(ctxs *[]Context) {
	// migrated from custom_type_array.java:2:5
	System.out.println(ctxs)
}
