package converted

type runner struct {
}

func newRunner() runner {
	this := runner{}
	return this
}

func (this *runner) Run() {
	System.out.println("Running once")
}

func (this *runner) RunWithInt(times int) {
	i := 0
	for ; i < times; i++ {
		System.out.println(("Running iteration " + i))
	}
}

func (this *runner) Test() {
	this.Run()
	this.RunWithInt(5)
}
