package converted

type processor struct {
}

func newProcessor() processor {
	this := processor{}
	return this
}

func (this *processor) Process(s string) {
	System.out.println(("String: " + s))
}

func (this *processor) ProcessWithInt(i int) {
	System.out.println(("Integer: " + i))
}

func (this *processor) Test() {
	// FIXME: more than one possible method for process with 1 arguments

	this.Process("test")
}
