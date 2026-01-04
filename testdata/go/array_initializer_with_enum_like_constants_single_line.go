package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) test() {
	// migrated from array_initializer_with_enum_like_constants_single_line.java:2:5
	alternatives := []Context{Context_START, Context_END}
}
