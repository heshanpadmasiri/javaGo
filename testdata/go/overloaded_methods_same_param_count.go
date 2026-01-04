package converted

type processor struct {
}

func newProcessor() processor {
	this := processor{}
	return this
}

func (this *processor) Process(s string) {
	// migrated from overloaded_methods_same_param_count.java:2:5
	System.out.println(("String: " + s))
}

func (this *processor) ProcessWithInt(i int) {
	// migrated from overloaded_methods_same_param_count.java:6:5
	System.out.println(("Integer: " + i))
}

func (this *processor) Test() {
	// migrated from overloaded_methods_same_param_count.java:10:5
	// FIXME: more than one possible method for process with 1 arguments

	this.Process("test")
}
