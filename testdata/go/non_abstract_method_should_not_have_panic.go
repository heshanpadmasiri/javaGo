package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) DoSomething() {
	// migrated from non_abstract_method_should_not_have_panic.java:2:5
	System.out.println("Hello")
}
